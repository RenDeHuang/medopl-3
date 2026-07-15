#!/usr/bin/env bash

patch_workspace_image() {
  local image="$1"
  local patch
  patch="$(WORKSPACE_IMAGE="$image" node -e '
    process.stdout.write(JSON.stringify({ data: { OPL_WORKSPACE_IMAGE: process.env.WORKSPACE_IMAGE } }));
  ')"
  kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" patch configmap opl-cloud-config \
    --type merge -p "$patch"
}

wait_workspace_rollouts() {
  local mode="$1"
  local failed=0
  local deployment container previous_image expected_image current_image
  [ -f "$workspace_images" ] || return 1
  while IFS=$'\t' read -r deployment container previous_image; do
    [ -n "$deployment" ] || continue
    if [ "$mode" = "previous" ]; then
      expected_image="$previous_image"
    else
      expected_image="$OPL_WORKSPACE_IMAGE"
    fi
    if ! kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" rollout status "deployment/$deployment" --timeout=300s; then
      failed=1
      continue
    fi
    if ! current_image="$(kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" get "deployment/$deployment" \
      -o "jsonpath={.spec.template.spec.containers[?(@.name=='$container')].image}")"; then
      failed=1
      continue
    fi
    [ "$current_image" = "$expected_image" ] || failed=1
  done < "$workspace_images"
  return "$failed"
}

wait_cloud_rollouts() {
  local mode="$1"
  local failed=0
  local deployment container expected_image current_image
  for deployment in opl-cloud-control-plane opl-cloud-ledger opl-cloud-fabric; do
    container="${deployment#opl-cloud-}"
    if [ "$mode" = "previous" ]; then
      if ! expected_image="$(cat "$rollback_dir/$deployment")"; then
        failed=1
        continue
      fi
    else
      expected_image="$OPL_CLOUD_IMAGE"
    fi
    if ! kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" rollout restart "deployment/$deployment"; then
      failed=1
      continue
    fi
    if ! kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" rollout status "deployment/$deployment" --timeout=300s; then
      failed=1
      continue
    fi
    if ! current_image="$(kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" get "deployment/$deployment" \
      -o "jsonpath={.spec.template.spec.containers[?(@.name=='$container')].image}")"; then
      failed=1
      continue
    fi
    [ "$current_image" = "$expected_image" ] || failed=1
  done
  return "$failed"
}

restore_previous_images() {
  local failed=0
  local item deployment container previous_image previous_workspace_image
  for item in \
    "opl-cloud-control-plane:control-plane" \
    "opl-cloud-ledger:ledger" \
    "opl-cloud-fabric:fabric"; do
    deployment="${item%%:*}"
    container="${item##*:}"
    if ! previous_image="$(cat "$rollback_dir/$deployment")"; then
      failed=1
      continue
    fi
    kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" set image "deployment/$deployment" "$container=$previous_image" || failed=1
  done
  if ! previous_workspace_image="$(cat "$rollback_dir/OPL_WORKSPACE_IMAGE")"; then
    failed=1
  else
    patch_workspace_image "$previous_workspace_image" || failed=1
  fi
  if [ ! -f "$workspace_images" ]; then
    failed=1
  else
    while IFS=$'\t' read -r deployment container previous_image; do
      [ -n "$deployment" ] || continue
      kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" set image "deployment/$deployment" "$container=$previous_image" || failed=1
    done < "$workspace_images"
  fi
  wait_cloud_rollouts previous || failed=1
  wait_workspace_rollouts previous || failed=1
  return "$failed"
}

apply_candidate_images() {
  local item deployment container previous_image
  patch_workspace_image "$OPL_WORKSPACE_IMAGE"
  for item in \
    "opl-cloud-control-plane:control-plane" \
    "opl-cloud-ledger:ledger" \
    "opl-cloud-fabric:fabric"; do
    deployment="${item%%:*}"
    container="${item##*:}"
    kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" set image "deployment/$deployment" "$container=$OPL_CLOUD_IMAGE"
  done
  while IFS=$'\t' read -r deployment container previous_image; do
    [ -n "$deployment" ] || continue
    kubectl --kubeconfig "$KUBECONFIG" -n "$OPL_K8S_NAMESPACE" set image "deployment/$deployment" "$container=$OPL_WORKSPACE_IMAGE"
  done < "$workspace_images"
  wait_cloud_rollouts candidate
  wait_workspace_rollouts candidate
}
