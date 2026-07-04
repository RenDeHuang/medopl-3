package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tke2022 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20220501"
)

var requiredTencentEnv = []string{
	"TENCENTCLOUD_SECRET_ID",
	"TENCENTCLOUD_SECRET_KEY",
	"TENCENTCLOUD_REGION",
	"TENCENT_DEPLOY_CLUSTER_ID",
}

type Request struct {
	Action     string                 `json:"action"`
	DryRun     bool                   `json:"dryRun,omitempty"`
	AccountId  string                 `json:"accountId,omitempty"`
	UserId     string                 `json:"userId,omitempty"`
	PackageId  string                 `json:"packageId,omitempty"`
	Pool       ComputePoolInput       `json:"pool,omitempty"`
	Allocation ComputeAllocationInput `json:"allocation,omitempty"`
}

type ComputePoolInput struct {
	Id                string            `json:"id,omitempty"`
	PackageId         string            `json:"packageId,omitempty"`
	InstanceType      string            `json:"instanceType,omitempty"`
	NodePoolId        string            `json:"nodePoolId,omitempty"`
	DesiredNodeLabels map[string]string `json:"desiredNodeLabels,omitempty"`
}

type ComputeAllocationInput struct {
	Id         string `json:"id,omitempty"`
	InstanceId string `json:"instanceId,omitempty"`
	NodeName   string `json:"nodeName,omitempty"`
}

type Response struct {
	Ok                bool              `json:"ok"`
	OperationId       string            `json:"operationId,omitempty"`
	PoolId            string            `json:"poolId,omitempty"`
	NodePoolId        string            `json:"nodePoolId,omitempty"`
	InstanceId        string            `json:"instanceId,omitempty"`
	NodeName          string            `json:"nodeName,omitempty"`
	Status            string            `json:"status,omitempty"`
	ProviderRequestId string            `json:"providerRequestId,omitempty"`
	ProviderData      map[string]string `json:"providerData,omitempty"`
	ErrorCode         string            `json:"errorCode,omitempty"`
	Message           string            `json:"message,omitempty"`
	Retryable         bool              `json:"retryable,omitempty"`
	MissingEnv        []string          `json:"missingEnv,omitempty"`
}

type TencentClient interface {
	CreateComputeAllocation(request Request, env map[string]string) Response
	DestroyComputeAllocation(request Request, env map[string]string) Response
}

type unimplementedTencentClient struct{}

type tencentSDKClient struct {
	region          string
	clusterId       string
	nativeTkeClient tkeNativeAPI
}

type tkeNativeAPI interface {
	CreateNodePool(request *tke2022.CreateNodePoolRequest) (*tke2022.CreateNodePoolResponse, error)
	DescribeNodePools(request *tke2022.DescribeNodePoolsRequest) (*tke2022.DescribeNodePoolsResponse, error)
	ScaleNodePool(request *tke2022.ScaleNodePoolRequest) (*tke2022.ScaleNodePoolResponse, error)
	DeleteClusterMachines(request *tke2022.DeleteClusterMachinesRequest) (*tke2022.DeleteClusterMachinesResponse, error)
}

func (unimplementedTencentClient) CreateComputeAllocation(_ Request, _ map[string]string) Response {
	return Response{
		Ok:        false,
		ErrorCode: "tencent_live_not_implemented",
		Message:   "Tencent live compute allocation is not implemented in this build.",
		Retryable: false,
	}
}

func (unimplementedTencentClient) DestroyComputeAllocation(_ Request, _ map[string]string) Response {
	return Response{
		Ok:        false,
		ErrorCode: "tencent_live_not_implemented",
		Message:   "Tencent live compute allocation destroy is not implemented in this build.",
		Retryable: false,
	}
}

func newTencentSDKClient(env map[string]string) (*tencentSDKClient, *Response) {
	missing := missingEnv(env)
	if len(missing) > 0 {
		return nil, &Response{
			Ok:         false,
			ErrorCode:  "tencent_env_missing",
			Message:    "Tencent Cloud provisioner environment is incomplete.",
			MissingEnv: missing,
			Retryable:  false,
		}
	}
	credential := common.NewCredential(env["TENCENTCLOUD_SECRET_ID"], env["TENCENTCLOUD_SECRET_KEY"])

	tkeProfile := profile.NewClientProfile()
	tkeProfile.HttpProfile.Endpoint = "tke.tencentcloudapi.com"
	tkeClient, err := tke2022.NewClient(credential, env["TENCENTCLOUD_REGION"], tkeProfile)
	if err != nil {
		return nil, &Response{
			Ok:        false,
			ErrorCode: "tencent_sdk_client_failed",
			Message:   err.Error(),
			Retryable: false,
		}
	}

	return &tencentSDKClient{
		region:          env["TENCENTCLOUD_REGION"],
		clusterId:       env["TENCENT_DEPLOY_CLUSTER_ID"],
		nativeTkeClient: tkeClient,
	}, nil
}

func (client *tencentSDKClient) CreateComputeAllocation(request Request, env map[string]string) Response {
	if client == nil || client.nativeTkeClient == nil {
		return Response{Ok: false, ErrorCode: "tencent_sdk_client_missing", Message: "Tencent TKE SDK client is missing.", Retryable: false}
	}
	nodePoolId := request.Pool.NodePoolId
	createNodePoolRequestId := ""
	if nodePoolId == "" {
		createNodePoolRequest, failure := buildCreateNativeNodePoolRequest(request, env)
		if failure != nil {
			return *failure
		}
		createNodePoolResponse, err := client.nativeTkeClient.CreateNodePool(createNodePoolRequest)
		if err != nil {
			return sdkErrorResponse("tencent_create_node_pool_failed", err)
		}
		nodePoolId = stringValue(createNodePoolResponse.Response.NodePoolId)
		createNodePoolRequestId = stringValue(createNodePoolResponse.Response.RequestId)
		if nodePoolId == "" {
			return Response{
				Ok:                false,
				ErrorCode:         "tencent_node_pool_id_missing",
				Message:           "Tencent TKE did not return a node pool id.",
				ProviderRequestId: createNodePoolRequestId,
				Retryable:         true,
			}
		}
	}

	pool, describeRequestId, err := client.describeNativeNodePool(nodePoolId)
	if err != nil {
		response := sdkErrorResponse("tencent_describe_node_pool_failed", err)
		response.ProviderRequestId = createNodePoolRequestId
		return response
	}
	currentReplicas := nativeReplicas(pool)
	targetReplicas := currentReplicas + 1
	scaleRequest := tke2022.NewScaleNodePoolRequest()
	scaleRequest.ClusterId = common.StringPtr(client.clusterId)
	scaleRequest.NodePoolId = common.StringPtr(nodePoolId)
	scaleRequest.Replicas = common.Int64Ptr(targetReplicas)
	scaleResponse, err := client.nativeTkeClient.ScaleNodePool(scaleRequest)
	if err != nil {
		response := sdkErrorResponse("tencent_scale_node_pool_failed", err)
		response.ProviderRequestId = describeRequestId
		return response
	}
	scaleRequestId := stringValue(scaleResponse.Response.RequestId)
	return Response{
		Ok:                true,
		OperationId:       "op-create-compute-" + stableSuffix(request.AccountId, request.Allocation.Id, nodePoolId, fmt.Sprintf("%d", targetReplicas))[:12],
		PoolId:            request.Pool.Id,
		NodePoolId:        nodePoolId,
		NodeName:          request.Allocation.NodeName,
		Status:            "provisioning",
		ProviderRequestId: scaleRequestId,
		ProviderData: map[string]string{
			"clusterId":                 client.clusterId,
			"region":                    client.region,
			"createNodePoolRequestId":   createNodePoolRequestId,
			"describeNodePoolRequestId": describeRequestId,
			"scaleNodePoolRequestId":    scaleRequestId,
			"instanceType":              request.Pool.InstanceType,
			"replicasBefore":            fmt.Sprintf("%d", currentReplicas),
			"replicasAfter":             fmt.Sprintf("%d", targetReplicas),
		},
	}
}

func (client *tencentSDKClient) DestroyComputeAllocation(request Request, _ map[string]string) Response {
	if client == nil || client.nativeTkeClient == nil {
		return Response{Ok: false, ErrorCode: "tencent_sdk_client_missing", Message: "Tencent TKE SDK client is missing.", Retryable: false}
	}
	if strings.TrimSpace(request.Pool.NodePoolId) == "" {
		return Response{Ok: false, ErrorCode: "node_pool_id_required", Message: "ComputePool nodePoolId is required.", Retryable: false}
	}
	providerRequestId := ""
	if strings.TrimSpace(request.Allocation.NodeName) != "" {
		deleteRequest := tke2022.NewDeleteClusterMachinesRequest()
		deleteRequest.ClusterId = common.StringPtr(client.clusterId)
		deleteRequest.MachineNames = []*string{common.StringPtr(request.Allocation.NodeName)}
		deleteRequest.EnableScaleDown = common.BoolPtr(true)
		deleteRequest.InstanceDeleteMode = common.StringPtr("terminate")
		deleteResponse, err := client.nativeTkeClient.DeleteClusterMachines(deleteRequest)
		if err != nil {
			return sdkErrorResponse("tencent_delete_cluster_machine_failed", err)
		}
		providerRequestId = stringValue(deleteResponse.Response.RequestId)
	} else {
		pool, describeRequestId, err := client.describeNativeNodePool(request.Pool.NodePoolId)
		if err != nil {
			return sdkErrorResponse("tencent_describe_node_pool_failed", err)
		}
		targetReplicas := nativeReplicas(pool) - 1
		if targetReplicas < 0 {
			targetReplicas = 0
		}
		scaleRequest := tke2022.NewScaleNodePoolRequest()
		scaleRequest.ClusterId = common.StringPtr(client.clusterId)
		scaleRequest.NodePoolId = common.StringPtr(request.Pool.NodePoolId)
		scaleRequest.Replicas = common.Int64Ptr(targetReplicas)
		scaleResponse, err := client.nativeTkeClient.ScaleNodePool(scaleRequest)
		if err != nil {
			response := sdkErrorResponse("tencent_scale_node_pool_failed", err)
			response.ProviderRequestId = describeRequestId
			return response
		}
		providerRequestId = stringValue(scaleResponse.Response.RequestId)
	}
	return Response{
		Ok:                true,
		OperationId:       "op-destroy-compute-" + stableSuffix(request.AccountId, request.Allocation.Id, request.Pool.NodePoolId, request.Allocation.NodeName)[:12],
		InstanceId:        request.Allocation.InstanceId,
		NodeName:          request.Allocation.NodeName,
		NodePoolId:        request.Pool.NodePoolId,
		Status:            "destroyed",
		ProviderRequestId: providerRequestId,
		ProviderData: map[string]string{
			"clusterId": client.clusterId,
			"region":    client.region,
		},
	}
}

func (client *tencentSDKClient) describeNativeNodePool(nodePoolId string) (*tke2022.NodePool, string, error) {
	describeRequest := tke2022.NewDescribeNodePoolsRequest()
	describeRequest.ClusterId = common.StringPtr(client.clusterId)
	describeRequest.Limit = common.Int64Ptr(100)
	describeRequest.Filters = []*tke2022.Filter{
		{Name: common.StringPtr("NodePoolsId"), Values: []*string{common.StringPtr(nodePoolId)}},
	}
	describeResponse, err := client.nativeTkeClient.DescribeNodePools(describeRequest)
	if err != nil {
		return nil, "", err
	}
	if len(describeResponse.Response.NodePools) == 0 {
		return nil, stringValue(describeResponse.Response.RequestId), fmt.Errorf("node pool not found: %s", nodePoolId)
	}
	return describeResponse.Response.NodePools[0], stringValue(describeResponse.Response.RequestId), nil
}

func buildCreateNativeNodePoolRequest(request Request, env map[string]string) (*tke2022.CreateNodePoolRequest, *Response) {
	missing := missingSpecificEnv(env, []string{
		"TENCENT_DEPLOY_CLUSTER_ID",
		"TENCENT_CVM_SUBNET_ID",
		"TENCENT_CVM_SECURITY_GROUP_IDS",
	})
	if len(missing) > 0 {
		return nil, &Response{
			Ok:         false,
			ErrorCode:  "tencent_node_pool_env_missing",
			Message:    "Tencent TKE node pool creation environment is incomplete.",
			MissingEnv: missing,
			Retryable:  false,
		}
	}
	nodePoolName := request.Pool.Id
	if strings.TrimSpace(nodePoolName) == "" {
		nodePoolName = "pool-" + request.PackageId + "-" + request.Pool.InstanceType
	}
	if strings.TrimSpace(request.Pool.InstanceType) == "" {
		return nil, &Response{Ok: false, ErrorCode: "instance_type_required", Message: "ComputePool instanceType is required.", Retryable: false}
	}
	createRequest := tke2022.NewCreateNodePoolRequest()
	createRequest.ClusterId = common.StringPtr(env["TENCENT_DEPLOY_CLUSTER_ID"])
	createRequest.Name = common.StringPtr(nodePoolName)
	createRequest.Type = common.StringPtr("Native")
	createRequest.DeletionProtection = common.BoolPtr(true)
	nodePoolLabels := []*tke2022.Label{
		{Name: common.StringPtr("oplcloud.cn/pool-id"), Value: common.StringPtr(request.Pool.Id)},
		{Name: common.StringPtr("oplcloud.cn/package-id"), Value: common.StringPtr(request.PackageId)},
		{Name: common.StringPtr("oplcloud.cn/instance-type"), Value: common.StringPtr(request.Pool.InstanceType)},
	}
	createRequest.Labels = nodePoolLabels
	createRequest.Native = &tke2022.CreateNativeNodePoolParam{
		Scaling: &tke2022.MachineSetScaling{
			MinReplicas:  common.Int64Ptr(0),
			MaxReplicas:  common.Int64Ptr(10),
			CreatePolicy: common.StringPtr("ZonePriority"),
		},
		SubnetIds:          stringsToPtrs(splitCsv(env["TENCENT_CVM_SUBNET_ID"])),
		InstanceChargeType: common.StringPtr("POSTPAID_BY_HOUR"),
		SystemDisk: &tke2022.Disk{
			DiskType: common.StringPtr(defaultString(env["TENCENT_CVM_SYSTEM_DISK_TYPE"], "CLOUD_BSSD")),
			DiskSize: common.Int64Ptr(int64(intFromEnv(env, "TENCENT_CVM_SYSTEM_DISK_SIZE_GB", 50))),
		},
		InstanceTypes:      []*string{common.StringPtr(request.Pool.InstanceType)},
		SecurityGroupIds:   stringsToPtrs(splitCsv(env["TENCENT_CVM_SECURITY_GROUP_IDS"])),
		AutoRepair:         common.BoolPtr(true),
		EnableAutoscaling:  common.BoolPtr(true),
		Replicas:           common.Int64Ptr(0),
		InternetAccessible: &tke2022.InternetAccessible{MaxBandwidthOut: common.Int64Ptr(0), ChargeType: common.StringPtr("TRAFFIC_POSTPAID_BY_HOUR")},
		MachineType:        common.StringPtr("Native"),
		AutomationService:  common.BoolPtr(true),
		RuntimeRootDir:     common.StringPtr("/var/lib/containerd"),
	}
	return createRequest, nil
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeResponse(Response{Ok: false, ErrorCode: "stdin_read_failed", Message: err.Error()})
		os.Exit(1)
	}
	var request Request
	if err := json.Unmarshal(raw, &request); err != nil {
		writeResponse(Response{Ok: false, ErrorCode: "invalid_json", Message: err.Error()})
		os.Exit(1)
	}
	env := envMap(os.Environ())
	client, setupFailure := newTencentSDKClient(env)
	if setupFailure != nil && request.Action != "readiness" {
		writeResponse(*setupFailure)
		os.Exit(1)
	}
	var provisioner TencentClient = client
	if provisioner == nil {
		provisioner = unimplementedTencentClient{}
	}
	response := handleWithClient(request, env, provisioner)
	writeResponse(response)
	if !response.Ok {
		os.Exit(1)
	}
}

func writeResponse(response Response) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(response)
}

func envMap(values []string) map[string]string {
	result := map[string]string{}
	for _, item := range values {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

func handle(request Request, env map[string]string) Response {
	return handleWithClient(request, env, unimplementedTencentClient{})
}

func handleWithClient(request Request, env map[string]string, client TencentClient) Response {
	missing := missingEnv(env)
	if request.Action == "readiness" {
		if len(missing) > 0 {
			return Response{
				Ok:         false,
				ErrorCode:  "tencent_env_missing",
				Message:    "Tencent Cloud provisioner environment is incomplete.",
				MissingEnv: missing,
				Retryable:  false,
			}
		}
		return Response{Ok: true, Status: "ready"}
	}
	if len(missing) > 0 {
		return Response{
			Ok:         false,
			ErrorCode:  "tencent_env_missing",
			Message:    "Tencent Cloud provisioner environment is incomplete.",
			MissingEnv: missing,
			Retryable:  false,
		}
	}
	if isLiveMutation(request) && strings.TrimSpace(env["RUN_TENCENT_CREATE_RELEASE_EXECUTION"]) != "1" {
		return Response{
			Ok:        false,
			ErrorCode: "live_mutation_flag_required",
			Message:   "Set RUN_TENCENT_CREATE_RELEASE_EXECUTION=1 to run live Tencent compute mutations.",
			Retryable: false,
		}
	}

	switch request.Action {
	case "create_compute_allocation":
		if request.DryRun {
			return dryRunCreateComputeAllocation(request, env)
		}
		return client.CreateComputeAllocation(request, env)
	case "destroy_compute_allocation":
		if request.DryRun {
			return dryRunDestroyComputeAllocation(request)
		}
		return client.DestroyComputeAllocation(request, env)
	default:
		return Response{
			Ok:        false,
			ErrorCode: "unknown_action",
			Message:   fmt.Sprintf("Unknown provisioner action: %s", request.Action),
			Retryable: false,
		}
	}
}

func isLiveMutation(request Request) bool {
	if request.DryRun {
		return false
	}
	return request.Action == "create_compute_allocation" || request.Action == "destroy_compute_allocation"
}

func missingEnv(env map[string]string) []string {
	var missing []string
	for _, key := range requiredTencentEnv {
		if strings.TrimSpace(env[key]) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func dryRunCreateComputeAllocation(request Request, env map[string]string) Response {
	stable := stableSuffix(request.AccountId, request.UserId, request.PackageId, request.Pool.Id, request.Allocation.Id)
	nodePoolId := request.Pool.NodePoolId
	if nodePoolId == "" {
		nodePoolId = "np-" + stable[:8]
	}
	instanceId := request.Allocation.InstanceId
	if instanceId == "" {
		instanceId = "ins-" + stable[:12]
	}
	nodeName := request.Allocation.NodeName
	if nodeName == "" {
		nodeName = "node-" + stable[:10]
	}
	return Response{
		Ok:          true,
		OperationId: "op-create-compute-" + stable[:12],
		PoolId:      request.Pool.Id,
		NodePoolId:  nodePoolId,
		InstanceId:  instanceId,
		NodeName:    nodeName,
		Status:      "provisioning",
		ProviderData: map[string]string{
			"accountId":       request.AccountId,
			"userId":          request.UserId,
			"packageId":       request.PackageId,
			"clusterId":       env["TENCENT_DEPLOY_CLUSTER_ID"],
			"region":          env["TENCENTCLOUD_REGION"],
			"instanceType":    request.Pool.InstanceType,
			"provisionerMode": "dry-run",
		},
	}
}

func dryRunDestroyComputeAllocation(request Request) Response {
	stable := stableSuffix(request.AccountId, request.Allocation.Id, request.Allocation.InstanceId)
	return Response{
		Ok:          true,
		OperationId: "op-destroy-compute-" + stable[:12],
		PoolId:      request.Pool.Id,
		NodePoolId:  request.Pool.NodePoolId,
		InstanceId:  request.Allocation.InstanceId,
		NodeName:    request.Allocation.NodeName,
		Status:      "destroyed",
		ProviderData: map[string]string{
			"accountId":       request.AccountId,
			"provisionerMode": "dry-run",
		},
	}
}

func stableSuffix(parts ...string) string {
	hash := sha1.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func missingSpecificEnv(env map[string]string, keys []string) []string {
	var missing []string
	for _, key := range keys {
		if strings.TrimSpace(env[key]) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func intFromEnv(env map[string]string, key string, fallback int) int {
	if strings.TrimSpace(env[key]) == "" {
		return fallback
	}
	value, err := strconv.Atoi(env[key])
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func splitCsv(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func stringsToPtrs(values []string) []*string {
	if len(values) == 0 {
		return nil
	}
	result := make([]*string, 0, len(values))
	for _, value := range values {
		result = append(result, common.StringPtr(value))
	}
	return result
}

func nativeReplicas(pool *tke2022.NodePool) int64 {
	if pool == nil || pool.Native == nil || pool.Native.Replicas == nil {
		return 0
	}
	return *pool.Native.Replicas
}

func compactName(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		isAlphaNum := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')
		if isAlphaNum {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(strings.TrimSpace(builder.String()), "-")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstString(values []*string) string {
	if len(values) == 0 || values[0] == nil {
		return ""
	}
	return *values[0]
}

func sdkErrorResponse(code string, err error) Response {
	return Response{
		Ok:        false,
		ErrorCode: code,
		Message:   err.Error(),
		Retryable: true,
	}
}
