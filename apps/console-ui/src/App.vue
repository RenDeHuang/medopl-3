<script setup lang="ts">
import {
  Activity,
  AlertCircle,
  ArrowUpRight,
  CalendarDays,
  Check,
  ChevronRight,
  CircleDollarSign,
  Copy,
  Database,
  Eye,
  EyeOff,
  HardDrive,
  KeyRound,
  LayoutDashboard,
  Link2,
  LogOut,
  Menu,
  MonitorCog,
  Network,
  Plus,
  ReceiptText,
  RefreshCw,
  Server,
  ShieldCheck,
  UserRound,
  UsersRound,
  WalletCards,
  X
} from "@lucide/vue";
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";

import { currentSession, login, logoutLocalFirst } from "./api/auth-api.ts";
import {
  createUser,
  getBillingReceipts,
  getConsoleState,
  getGatewaySummary,
  getManagementState,
  getOperatorSummary,
  getPricingCatalog,
  previewPricing
} from "./api/console-read-api.ts";
import { attachStorage, createComputeAllocation, createStorageVolume } from "./api/resources-api.ts";
import {
  createWorkspace,
  createWorkspaceLaunchIntent,
  launchWorkspaceResource
} from "./api/workspaces-api.ts";
import {
  customerBillingStatusLabel,
  customerMenu,
  fixedMonthlySpend,
  formatAvailableBalance,
  formatDate,
  formatUsdMicros,
  gatewayCanCall,
  maskGatewaySummary,
  needsSession,
  resourceOrderStage,
  resourceNeedsAttention,
  resourceStatusLabel,
  renewalSummary,
  storageMonthlyPrice,
  workspaceMonthlyPrice,
  workspaceProgress
} from "./console-model.ts";

type AnyRecord = Record<string, any>;

const menuIcons: Record<string, any> = { LayoutDashboard, MonitorCog, Database, Network, ReceiptText };
const adminMenu = [
  { label: "管理概览", path: "/admin/overview", icon: LayoutDashboard },
  { label: "用户", path: "/admin/users", icon: UsersRound }
];
const orderStages = ["已受理", "订单已确认", "云端开通中", "可用"];

const path = ref(window.location.pathname);
const session = ref<AnyRecord | null>(null);
const authStatus = ref(needsSession(path.value) ? "checking" : "public");
const authError = ref("");
const state = ref<AnyRecord | null>(null);
const gateway = ref<AnyRecord | null>(null);
const receipts = ref<AnyRecord[]>([]);
const receiptPage = reactive({ nextCursor: "", hasMore: false });
const catalog = ref<AnyRecord | null>(null);
const storageQuotes = ref<AnyRecord>({});
const management = ref<AnyRecord | null>(null);
const operatorSummary = ref<AnyRecord | null>(null);
const errors = reactive({ state: "", gateway: "", receipts: "", catalog: "", admin: "" });
const loading = reactive({ state: false, gateway: false, receipts: false, catalog: false, admin: false });
const sidebarOpen = ref(false);
const gatewayTab = ref("overview");
const gatewayBusy = ref(false);
const mutationBusy = ref(false);
const modal = ref("");
const toast = reactive({ text: "", tone: "good" });
const launchStatus = ref("");
const loginForm = reactive({ email: "", password: "" });
const loginBusy = ref(false);
const loginError = ref("");
const launchForm = reactive({ workspaceName: "", computeName: "", storageName: "", packageId: "basic" });
const computeForm = reactive({ name: "", packageId: "basic" });
const storageForm = reactive({ name: "", packageId: "basic", computeAllocationId: "" });
const adminUserForm = reactive({ email: "", password: "", name: "", accountId: "", sub2apiUserId: "" });
let launchIntent: AnyRecord | null = null;
let toastTimer: number | undefined;
let pollTimer: number | undefined;

const isAdminRoute = computed(() => path.value === "/admin" || path.value.startsWith("/admin/"));
const isLoginRoute = computed(() => path.value === "/login");
const isForbidden = computed(() => path.value === "/403");
const isPublicRoute = computed(() => !needsSession(path.value) && !isLoginRoute.value && !isForbidden.value);
const isOperator = computed(() => session.value?.isOperator === true);
const plans = computed(() => (catalog.value?.packages || []).filter((item) => item.available && ["basic", "pro"].includes(item.id)));
const workspaces = computed(() => state.value?.workspaces || []);
const workspace = computed(() => workspaces.value[0] || null);
const computes = computed(() => (state.value?.computeAllocations || []).filter((item) => !["destroyed", "deleted"].includes(item.status)));
const storages = computed(() => (state.value?.storageVolumes || []).filter((item) => !["destroyed", "deleted"].includes(item.status)));
const attachments = computed(() => (state.value?.storageAttachments || []).filter((item) => !["detached", "deleted"].includes(item.status)));
const workspaceSteps = computed(() => workspaceProgress(state.value || {}, workspace.value || {}));
const allBillableResources = computed(() => [...computes.value, ...storages.value]);
const currentFixedSpend = computed(() => fixedMonthlySpend(allBillableResources.value));
const latestOrder = computed(() => [...allBillableResources.value]
  .sort((a, b) => new Date(b.createdAt || 0).getTime() - new Date(a.createdAt || 0).getTime())[0] || null);
const latestOrderStage = computed(() => resourceOrderStage(latestOrder.value || {}));
const balance = computed(() => gateway.value?.balance || state.value?.balance || {});
const gatewayUsage = computed(() => gateway.value?.usage || {});
const gatewayKey = computed(() => gateway.value?.apiKey || {});
const gatewayHealthy = computed(() => gatewayCanCall(gateway.value || {}));
const hasPendingResources = computed(() => allBillableResources.value.some((item) =>
  ["provisioning", "attaching", "destroying", "detaching"].includes(item.status)
  || ["preparing", "charge_pending", "provider_pending"].includes(item.billingStatus)
));
const selectedLaunchPlan = computed(() => plans.value.find((item) => item.id === launchForm.packageId) || plans.value[0]);
const selectedStoragePlan = computed(() => plans.value.find((item) => item.id === storageForm.packageId) || plans.value[0]);
const currentAttachment = computed(() => attachments.value.find((item) =>
  item.id === workspace.value?.currentAttachmentId || item.id === workspace.value?.attachmentId
) || attachments.value[0] || null);
const currentCompute = computed(() => computes.value.find((item) =>
  item.id === currentAttachment.value?.computeAllocationId
  || item.id === workspace.value?.currentComputeAllocationId
  || item.id === workspace.value?.computeAllocationId
) || computes.value[0] || null);
const currentStorage = computed(() => storages.value.find((item) =>
  item.id === currentAttachment.value?.storageId || item.id === workspace.value?.storageId
) || storages.value.find((item) => !currentCompute.value || item.computeAllocationId === currentCompute.value.id) || storages.value[0] || null);
const pageTitle = computed(() => {
  if (path.value.startsWith("/console/compute")) return "计算";
  if (path.value.startsWith("/console/storage")) return "存储";
  if (path.value.startsWith("/console/gateway")) return "Gateway";
  if (path.value.startsWith("/console/billing")) return "账单";
  if (path.value.startsWith("/admin/users")) return "用户";
  if (isAdminRoute.value) return "管理概览";
  return "概览";
});

function navigate(next: string) {
  window.history.pushState({}, "", next);
  path.value = window.location.pathname;
  sidebarOpen.value = false;
  void handleRoute();
}

function openWorkspace() {
  if (workspace.value?.openable && workspace.value.url) {
    window.open(workspace.value.url, "_blank", "noopener,noreferrer");
  }
}

function flash(text: string, tone = "good") {
  toast.text = text;
  toast.tone = tone;
  if (toastTimer) window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => { toast.text = ""; }, 3200);
}

function friendlyError(error: any) {
  const raw = String(error?.message || error || "操作失败");
  const messages: Record<string, string> = {
    not_authenticated: "登录已失效，请重新登录",
    account_scope_forbidden: "没有权限访问该资源",
    insufficient_balance: "可用余额不足",
    gateway_key_missing: "Gateway Key 尚未就绪",
    gateway_key_ambiguous: "Gateway Key 状态异常，请联系管理员",
    monthly_account_unmapped: "Gateway 账户尚未开通",
    upstream_unavailable: "服务暂不可用，请稍后重试"
  };
  return messages[raw] || (raw.includes("failed") || raw.includes("_") ? "请求失败，请重试" : raw);
}

async function loadState() {
  loading.state = true;
  errors.state = "";
  try {
    state.value = await getConsoleState(session.value?.user?.accountId || "");
  } catch (error) {
    errors.state = friendlyError(error);
  } finally {
    loading.state = false;
  }
}

async function loadGateway() {
  loading.gateway = true;
  errors.gateway = "";
  try {
    gateway.value = await getGatewaySummary(false);
  } catch (error) {
    errors.gateway = friendlyError(error);
  } finally {
    loading.gateway = false;
  }
}

async function loadReceipts(reset = true) {
  loading.receipts = true;
  errors.receipts = "";
  try {
    const page = await getBillingReceipts(reset ? "" : receiptPage.nextCursor, 20);
    receipts.value = reset ? (page.receipts || []) : [...receipts.value, ...(page.receipts || [])];
    receiptPage.nextCursor = page.nextCursor || "";
    receiptPage.hasMore = page.hasMore === true;
  } catch (error) {
    errors.receipts = friendlyError(error);
  } finally {
    loading.receipts = false;
  }
}

async function loadCatalog() {
  loading.catalog = true;
  errors.catalog = "";
  try {
    const nextCatalog = await getPricingCatalog();
    catalog.value = nextCatalog;
    const quotes = await Promise.all((nextCatalog.packages || [])
      .filter((plan) => ["basic", "pro"].includes(plan.id))
      .map(async (plan) => [plan.id, await previewPricing({
        resourceType: "storage",
        packageId: plan.id,
        sizeGb: plan.diskGb
      }, session.value?.csrfToken)]));
    storageQuotes.value = Object.fromEntries(quotes);
  } catch (error) {
    errors.catalog = friendlyError(error);
  } finally {
    loading.catalog = false;
  }
}

async function loadConsole() {
  await Promise.all([loadState(), loadGateway(), loadReceipts(), loadCatalog()]);
}

async function loadAdmin() {
  loading.admin = true;
  errors.admin = "";
  try {
    const [managementState, summary] = await Promise.all([getManagementState(), getOperatorSummary()]);
    management.value = managementState;
    operatorSummary.value = summary;
  } catch (error) {
    errors.admin = friendlyError(error);
  } finally {
    loading.admin = false;
  }
}

async function ensureSession() {
  authStatus.value = "checking";
  authError.value = "";
  try {
    const payload = await currentSession();
    if (!payload) {
      const redirect = encodeURIComponent(window.location.pathname + window.location.search);
      navigate(`/login?redirect=${redirect}`);
      return false;
    }
    session.value = payload;
    if (isAdminRoute.value && payload.isOperator !== true) {
      navigate("/403");
      return false;
    }
    authStatus.value = "ready";
    return true;
  } catch (error) {
    authError.value = friendlyError(error);
    authStatus.value = "error";
    return false;
  }
}

async function handleRoute() {
  if (!needsSession(path.value)) {
    authStatus.value = "public";
    return;
  }
  if (!session.value && !(await ensureSession())) return;
  if (isAdminRoute.value && !isOperator.value) {
    navigate("/403");
    return;
  }
  authStatus.value = "ready";
  if (isAdminRoute.value) {
    if (!management.value) await loadAdmin();
  } else if (!state.value) {
    await loadConsole();
  }
}

async function submitLogin() {
  loginBusy.value = true;
  loginError.value = "";
  try {
    const payload = await login(loginForm);
    session.value = payload;
    authStatus.value = "ready";
    const requested = new URLSearchParams(window.location.search).get("redirect");
    const target = requested?.startsWith("/") ? requested : payload.isOperator === true ? "/admin/overview" : "/console/overview";
    navigate(target);
  } catch (error) {
    loginError.value = friendlyError(error);
  } finally {
    loginBusy.value = false;
  }
}

async function signOut() {
  const csrf = session.value?.csrfToken || "";
  try {
    await logoutLocalFirst(csrf, () => {
      session.value = null;
      state.value = null;
      gateway.value = null;
      management.value = null;
    }, () => navigate("/"));
  } catch {
    // Local logout already completed.
  }
}

async function runMutation(action: () => Promise<any>, success: string) {
  if (mutationBusy.value) return null;
  mutationBusy.value = true;
  try {
    const result = await action();
    await loadState();
    flash(success);
    return result;
  } catch (error) {
    flash(friendlyError(error), "danger");
    return null;
  } finally {
    mutationBusy.value = false;
  }
}

function computeReady(resource: AnyRecord | null) {
  return resource?.billingStatus === "active" && ["running", "ready", "active"].includes(resource.status);
}

function storageReady(resource: AnyRecord | null) {
  return resource?.billingStatus === "active" && ["bound", "available", "ready", "active"].includes(resource.status);
}

function failedResource(resource: AnyRecord | null) {
  return !resource || resource.ok === false || resourceNeedsAttention(resource);
}

function openModal(name: string) {
  modal.value = name;
  if (name === "workspace") {
    launchForm.workspaceName ||= workspace.value?.name || "";
    launchForm.computeName ||= currentCompute.value?.name || "";
    launchForm.storageName ||= currentStorage.value?.name || "";
    launchForm.packageId = currentCompute.value?.packageId || launchForm.packageId || "basic";
  }
  if (name === "storage") storageForm.computeAllocationId ||= computes.value[0]?.id || "";
}

async function launchWorkspace() {
  if (!selectedLaunchPlan.value) return;
  launchStatus.value = "";
  const accountId = session.value?.user?.accountId || "";
  const requested = launchIntent?.input || {
    workspaceName: launchForm.workspaceName.trim(),
    computeName: launchForm.computeName.trim(),
    storageName: launchForm.storageName.trim(),
    packageId: launchForm.packageId,
    sizeGb: Number(selectedLaunchPlan.value.diskGb)
  };
  launchIntent = createWorkspaceLaunchIntent(requested, launchIntent, accountId);

  const computeStep = await launchWorkspaceResource(
    currentCompute.value,
    () => runMutation(() => createComputeAllocation({
      name: launchIntent!.input.computeName,
      packageId: launchIntent!.input.packageId
    }, session.value?.csrfToken, launchIntent!.idempotencyKeys.compute), "计算资源状态已更新"),
    computeReady
  );
  if (failedResource(computeStep.resource)) return;
  if (!computeStep.ready) {
    launchStatus.value = "计算资源正在开通，请稍后继续";
    return;
  }

  const storageStep = await launchWorkspaceResource(
    currentStorage.value?.computeAllocationId && currentStorage.value.computeAllocationId !== computeStep.resource.id ? null : currentStorage.value,
    () => runMutation(() => createStorageVolume({
      name: launchIntent!.input.storageName,
      packageId: launchIntent!.input.packageId,
      sizeGb: launchIntent!.input.sizeGb,
      computeAllocationId: computeStep.resource.id
    }, session.value?.csrfToken, launchIntent!.idempotencyKeys.storage), "存储资源状态已更新"),
    storageReady
  );
  if (failedResource(storageStep.resource)) return;
  if (!storageStep.ready) {
    launchStatus.value = "存储资源正在开通，请稍后继续";
    return;
  }

  let attachment = currentAttachment.value;
  if (attachment && (attachment.computeAllocationId !== computeStep.resource.id || attachment.storageId !== storageStep.resource.id)) attachment = null;
  if (!attachment) {
    attachment = await runMutation(() => attachStorage({
      computeAllocationId: computeStep.resource.id,
      storageId: storageStep.resource.id,
      mountPath: "/data"
    }, session.value?.csrfToken, launchIntent!.idempotencyKeys.attachment), "存储已挂载");
  }
  if (!attachment || failedResource(attachment)) return;

  const created = await runMutation(() => createWorkspace({
    input: { workspaceName: launchIntent!.input.workspaceName, attachmentId: attachment.id },
    idempotencyKey: launchIntent!.idempotencyKeys.workspace
  }, session.value?.csrfToken), "Workspace 开通请求已完成");
  if (!created || failedResource(created)) return;
  launchIntent = null;
  launchStatus.value = "Workspace 正在启动";
  modal.value = "";
}

async function buyCompute() {
  const result = await runMutation(() => createComputeAllocation({
    name: computeForm.name.trim(),
    packageId: computeForm.packageId
  }, session.value?.csrfToken, crypto.randomUUID()), "计算资源开通请求已提交");
  if (result) modal.value = "";
}

async function buyStorage() {
  const result = await runMutation(() => createStorageVolume({
    name: storageForm.name.trim(),
    packageId: storageForm.packageId,
    sizeGb: Number(selectedStoragePlan.value?.diskGb || 0),
    computeAllocationId: storageForm.computeAllocationId
  }, session.value?.csrfToken, crypto.randomUUID()), "存储资源开通请求已提交");
  if (result) modal.value = "";
}

async function mountStorage(storage: AnyRecord) {
  const computeId = storage.computeAllocationId || currentCompute.value?.id || computes.value[0]?.id;
  if (!computeId) return flash("请先开通计算资源", "danger");
  await runMutation(() => attachStorage({
    computeAllocationId: computeId,
    storageId: storage.id,
    mountPath: "/data"
  }, session.value?.csrfToken, crypto.randomUUID()), "存储已挂载");
}

async function revealGatewayKey() {
  gatewayBusy.value = true;
  errors.gateway = "";
  try {
    gateway.value = await getGatewaySummary(true);
  } catch (error) {
    errors.gateway = friendlyError(error);
  } finally {
    gatewayBusy.value = false;
  }
}

function hideGatewayKey() {
  gateway.value = maskGatewaySummary(gateway.value);
}

async function copyGatewayKey() {
  if (!gatewayKey.value.value) return;
  try {
    await navigator.clipboard.writeText(gatewayKey.value.value);
    flash("API Key 已复制");
  } catch {
    flash("复制失败，请重试", "danger");
  }
}

async function createCustomerUser() {
  if (mutationBusy.value) return;
  mutationBusy.value = true;
  try {
    await createUser({
      ...adminUserForm,
      role: "owner",
      sub2apiUserId: Number(adminUserForm.sub2apiUserId)
    }, session.value?.csrfToken);
    await loadAdmin();
    Object.assign(adminUserForm, { email: "", password: "", name: "", accountId: "", sub2apiUserId: "" });
    modal.value = "";
    flash("用户已创建");
  } catch (error) {
    flash(friendlyError(error), "danger");
  } finally {
    mutationBusy.value = false;
  }
}

function planFor(resource: AnyRecord) {
  return plans.value.find((plan) => plan.id === resource.packageId);
}

function attachmentFor(storage: AnyRecord) {
  return attachments.value.find((item) => item.storageId === storage.id && ["attached", "ready"].includes(item.status));
}

function receiptTitle(receipt: AnyRecord) {
  if (receipt.type?.includes("refund")) return "资源退款";
  if (receipt.type?.includes("renew")) return "资源续费";
  if (receipt.type?.includes("review")) return "账单处理";
  if (receipt.type?.includes("purchased") || receipt.type?.includes("charged")) return "资源购买";
  return "账单记录";
}

function resourceTypeLabel(value: any) {
  return String(value || "").includes("storage") ? "存储" : String(value || "").includes("compute") ? "计算" : "资源";
}

watch(hasPendingResources, (pending) => {
  if (pollTimer) window.clearInterval(pollTimer);
  pollTimer = pending ? window.setInterval(() => { void loadState(); }, 10_000) : undefined;
}, { immediate: true });

watch(path, (next, previous) => {
  if (previous.startsWith("/console/gateway") && !next.startsWith("/console/gateway")) hideGatewayKey();
});

onMounted(() => {
  const onPopState = () => {
    path.value = window.location.pathname;
    void handleRoute();
  };
  window.addEventListener("popstate", onPopState);
  (window as any).__oplPopState = onPopState;
  void handleRoute();
});

onBeforeUnmount(() => {
  window.removeEventListener("popstate", (window as any).__oplPopState);
  if (pollTimer) window.clearInterval(pollTimer);
  if (toastTimer) window.clearTimeout(toastTimer);
});
</script>

<template>
  <main v-if="isPublicRoute" class="access-page">
    <nav class="public-nav">
      <a href="/" class="brand" @click.prevent="navigate('/')">
        <img src="/opl-app-icon.png" alt="" />
        <strong>OPL Cloud</strong>
      </a>
      <button class="button secondary" type="button" @click="navigate('/login')">登录</button>
    </nav>
    <section class="access-main">
      <div>
        <p class="kicker">One Person Lab</p>
        <h1>OPL Cloud</h1>
        <p>科研 Workspace、计算、存储与 Gateway。</p>
        <button class="button primary" type="button" @click="navigate('/login')">
          进入 Console <ArrowUpRight :size="17" />
        </button>
      </div>
      <img class="access-mark" src="/opl-app-icon.png" alt="OPL Cloud" />
    </section>
  </main>

  <main v-else-if="isLoginRoute" class="login-page">
    <button class="back-button" type="button" @click="navigate('/')">返回</button>
    <section class="login-panel">
      <div class="login-brand">
        <img src="/opl-app-icon.png" alt="" />
        <div><strong>OPL Cloud</strong><span>Console 登录</span></div>
      </div>
      <form @submit.prevent="submitLogin">
        <label>邮箱<input v-model.trim="loginForm.email" type="email" autocomplete="username" required /></label>
        <label>密码<input v-model="loginForm.password" type="password" autocomplete="current-password" required /></label>
        <p v-if="loginError" class="form-error" role="alert">{{ loginError }}</p>
        <button class="button primary wide" type="submit" :disabled="loginBusy">
          {{ loginBusy ? "登录中..." : "登录" }}
        </button>
      </form>
    </section>
  </main>

  <main v-else-if="isForbidden" class="message-page">
    <ShieldCheck :size="34" />
    <h1>无权访问</h1>
    <p>此页面仅对 OPL Cloud 管理员开放。</p>
    <button class="button primary" type="button" @click="navigate('/console/overview')">返回 Console</button>
  </main>

  <main v-else-if="authStatus === 'checking'" class="message-page" aria-live="polite">
    <span class="spinner" />
    <p>正在恢复登录...</p>
  </main>

  <main v-else-if="authStatus === 'error'" class="message-page">
    <AlertCircle :size="34" />
    <h1>无法恢复登录</h1>
    <p>{{ authError }}</p>
    <button class="button primary" type="button" @click="ensureSession">重试</button>
  </main>

  <div v-else class="app-shell">
    <button class="mobile-menu" type="button" aria-label="打开导航" @click="sidebarOpen = true"><Menu /></button>
    <aside class="sidebar" :class="{ open: sidebarOpen }">
      <div class="sidebar-head">
        <a href="/console/overview" class="brand" @click.prevent="navigate(isOperator && isAdminRoute ? '/admin/overview' : '/console/overview')">
          <img src="/opl-app-icon.png" alt="" />
          <strong>OPL Console</strong>
        </a>
        <button class="sidebar-close" type="button" aria-label="关闭导航" @click="sidebarOpen = false"><X /></button>
      </div>
      <nav class="side-nav" aria-label="主导航">
        <template v-if="isAdminRoute">
          <a v-for="item in adminMenu" :key="item.path" :href="item.path" :class="{ active: path === item.path || (item.path.endsWith('overview') && path === '/admin') }" @click.prevent="navigate(item.path)">
            <component :is="item.icon" :size="19" />{{ item.label }}
          </a>
        </template>
        <template v-else>
          <a v-for="item in customerMenu" :key="item.path" :href="item.path" :class="{ active: path.startsWith(item.path.replace('/overview', '')) && (item.id !== 'overview' || path.endsWith('/overview')) }" @click.prevent="navigate(item.path)">
            <component :is="menuIcons[item.icon]" :size="19" />{{ item.label }}
          </a>
        </template>
      </nav>
      <div class="sidebar-account">
        <UserRound :size="18" />
        <span><strong>{{ session?.user?.email }}</strong><small>{{ isOperator ? "管理员" : "用户" }}</small></span>
        <button type="button" aria-label="退出登录" title="退出登录" @click="signOut"><LogOut :size="17" /></button>
      </div>
    </aside>
    <button v-if="sidebarOpen" class="sidebar-scrim" type="button" aria-label="关闭导航" @click="sidebarOpen = false" />

    <section class="main-column">
      <header class="topbar"><h1>{{ pageTitle }}</h1><button class="icon-button" type="button" title="刷新" aria-label="刷新" @click="isAdminRoute ? loadAdmin() : loadConsole()"><RefreshCw :size="17" /></button></header>

      <div v-if="isAdminRoute" class="page-content">
        <div v-if="loading.admin && !management" class="loading-panel"><span class="spinner" />正在加载管理数据...</div>
        <div v-else-if="errors.admin && !management" class="empty-panel"><AlertCircle /><p>{{ errors.admin }}</p><button class="button secondary" @click="loadAdmin">重试</button></div>
        <template v-else>
          <section v-if="path !== '/admin/users'" class="admin-dashboard">
            <div class="metric-row">
              <article><UsersRound /><span>用户<strong>{{ management?.users?.length || 0 }}</strong></span></article>
              <article><Server /><span>计算资源<strong>{{ management?.computeAllocations?.length || 0 }}</strong></span></article>
              <article><HardDrive /><span>存储资源<strong>{{ management?.storageVolumes?.length || 0 }}</strong></span></article>
              <article><Activity /><span>运行状态<strong>{{ operatorSummary?.runtimeReadiness?.ready === false ? "需处理" : "正常" }}</strong></span></article>
            </div>
            <section class="panel"><div class="panel-title"><h2>最近 Workspace</h2></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>账号</th><th>状态</th><th>创建时间</th></tr></thead><tbody><tr v-for="item in (management?.workspaces || []).slice(0, 10)" :key="item.id"><td>{{ item.name || "未命名" }}</td><td>{{ item.accountId }}</td><td><span class="status-pill">{{ item.openable ? "可用" : item.status || item.state }}</span></td><td>{{ formatDate(item.createdAt, true) }}</td></tr><tr v-if="!(management?.workspaces || []).length"><td colspan="4" class="empty-cell">暂无 Workspace</td></tr></tbody></table></div></section>
          </section>
          <section v-else class="panel">
            <div class="panel-title"><h2>用户</h2><button class="button primary" type="button" @click="modal = 'admin-user'"><Plus :size="16" />新建用户</button></div>
            <div class="table-wrap"><table><thead><tr><th>邮箱</th><th>账号</th><th>角色</th><th>状态</th></tr></thead><tbody><tr v-for="user in management?.users || []" :key="user.id"><td>{{ user.email }}</td><td>{{ user.accountId }}</td><td>{{ user.email?.toLowerCase() === 'admin@medopl.cn' ? "管理员" : "用户" }}</td><td><span class="status-pill" :class="{ good: user.status === 'active' }">{{ user.status === "active" ? "正常" : user.status }}</span></td></tr></tbody></table></div>
          </section>
        </template>
      </div>

      <div v-else class="page-content">
        <div v-if="loading.state && !state" class="loading-panel"><span class="spinner" />正在加载账号数据...</div>
        <div v-else-if="errors.state && !state" class="empty-panel"><AlertCircle /><p>{{ errors.state }}</p><button class="button secondary" @click="loadState">重试</button></div>

        <template v-else-if="state">
          <section v-if="path === '/console' || path.endsWith('/overview')" class="overview-layout">
            <div class="overview-main">
              <section class="panel workspace-panel">
                <div v-if="workspace" class="workspace-heading">
                  <div><span class="section-label">Workspace</span><h2>{{ workspace.name || "Workspace" }} <span class="status-pill" :class="{ good: workspace.openable }">{{ workspace.openable ? "可用" : resourceNeedsAttention(workspace) ? "需要处理" : "启动中" }}</span></h2></div>
                  <button class="button primary" type="button" :disabled="!workspace.openable || !workspace.url" @click="openWorkspace">打开 Workspace <ArrowUpRight :size="16" /></button>
                </div>
                <div v-else class="workspace-heading">
                  <div><span class="section-label">Workspace</span><h2>尚未开通</h2></div>
                  <button class="button primary" type="button" :disabled="!plans.length" @click="openModal('workspace')"><Plus :size="16" />开通 Workspace</button>
                </div>
                <ol class="workspace-progress" aria-label="Workspace 开通进度">
                  <li v-for="(step, index) in workspaceSteps" :key="step.label" :class="{ complete: step.complete }"><span><Check v-if="step.complete" :size="16" /><template v-else>{{ index + 1 }}</template></span><small>{{ step.label }}</small></li>
                </ol>
                <p v-if="launchStatus" class="inline-notice">{{ launchStatus }}</p>
              </section>

              <section class="panel">
                <div class="panel-title"><h2>资源列表</h2></div>
                <div class="table-wrap"><table><thead><tr><th>类型</th><th>名称</th><th>配置 / 路径</th><th>计划 / 价格</th><th>状态 / 到期时间</th></tr></thead><tbody>
                  <tr v-for="item in computes" :key="item.id"><td><span class="resource-type compute"><MonitorCog :size="16" />计算</span></td><td>{{ item.name || "未命名" }}</td><td>{{ planFor(item)?.cpu || "-" }}C / {{ planFor(item)?.memoryGb || "-" }}GB</td><td>{{ planFor(item)?.name || "-" }} · {{ formatUsdMicros(item.chargeUsdMicros) }}/月</td><td><span class="dot" :class="{ good: resourceStatusLabel(item) === '可用' }" />{{ resourceStatusLabel(item) }} · {{ formatDate(item.paidThrough) }}</td></tr>
                  <tr v-for="item in storages" :key="item.id"><td><span class="resource-type storage"><Database :size="16" />存储</span></td><td>{{ item.name || "未命名" }}</td><td>{{ item.sizeGb || "-" }}GB</td><td>{{ formatUsdMicros(item.chargeUsdMicros) }}/月</td><td><span class="dot" :class="{ good: resourceStatusLabel(item) === '可用' }" />{{ resourceStatusLabel(item) }} · {{ formatDate(item.paidThrough) }}</td></tr>
                  <tr v-for="item in attachments" :key="item.id"><td><span class="resource-type attachment"><Link2 :size="16" />挂载</span></td><td>{{ storages.find((storage) => storage.id === item.storageId)?.name || "未命名存储" }} → {{ computes.find((compute) => compute.id === item.computeAllocationId)?.name || "未命名计算" }}</td><td>{{ item.mountPath || "-" }}</td><td>-</td><td><span class="dot" :class="{ good: ['attached', 'ready'].includes(item.status) }" />{{ ['attached', 'ready'].includes(item.status) ? "已挂载" : resourceNeedsAttention(item) ? "需要处理" : "挂载中" }}</td></tr>
                  <tr v-if="!computes.length && !storages.length && !attachments.length"><td colspan="5" class="empty-cell">暂无资源</td></tr>
                </tbody></table></div>
              </section>

              <section class="spend-strip"><div><WalletCards /><span>当前固定月支出<strong>{{ formatUsdMicros(currentFixedSpend) }}</strong></span></div><div><RefreshCw /><span>续费状态<strong>{{ renewalSummary(allBillableResources) }}</strong></span></div></section>

              <section class="panel order-panel">
                <div class="panel-title"><h2>最近订单</h2><span v-if="latestOrder">{{ latestOrder.name || "未命名资源" }}</span></div>
                <ol v-if="latestOrder" class="order-progress"><li v-for="(label, index) in orderStages" :key="label" :class="{ complete: latestOrderStage >= index + 1 }"><span><Check v-if="latestOrderStage >= index + 1" :size="15" /><template v-else>{{ index + 1 }}</template></span><small>{{ label }}</small></li></ol>
                <p v-else class="empty-copy">暂无订单</p>
              </section>
            </div>

            <aside class="overview-rail panel">
              <div><WalletCards /><span>可用余额<strong>{{ formatAvailableBalance(balance) }}</strong></span></div>
              <div><Activity /><span>近 7 天 AI 用量<strong>{{ errors.gateway ? "暂不可用" : formatUsdMicros(gatewayUsage.usage7dUsdMicros) }}</strong></span></div>
              <div><ShieldCheck /><span>API 调用<strong :class="gatewayHealthy ? 'positive' : ''">{{ errors.gateway ? "暂不可用" : gatewayHealthy ? "正常" : "需要处理" }}</strong></span></div>
              <button type="button" @click="navigate('/console/gateway')"><Network /><span>管理 Gateway</span><ChevronRight /></button>
            </aside>
          </section>

          <section v-else-if="path.startsWith('/console/compute')" class="resource-page">
            <div class="page-actions"><div><h2>计算资源</h2><p>Basic 与 Pro，按月开通。</p></div><button class="button primary" type="button" :disabled="!plans.length" @click="openModal('compute')"><Plus :size="16" />开通计算</button></div>
            <div class="plan-grid"><article v-for="plan in plans" :key="plan.id" class="plan-panel"><span class="plan-name">{{ plan.name }}</span><strong>{{ formatUsdMicros(plan.price?.chargeUsdMicros) }}<small>/月</small></strong><p>{{ plan.cpu }} vCPU · {{ plan.memoryGb }}GB 内存</p><button class="button secondary wide" type="button" @click="computeForm.packageId = plan.id; openModal('compute')">选择 {{ plan.name }}</button></article></div>
            <section class="panel"><div class="panel-title"><h2>我的计算资源</h2></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>规格</th><th>价格</th><th>购买时间</th><th>到期时间</th><th>状态</th></tr></thead><tbody><tr v-for="item in computes" :key="item.id"><td>{{ item.name || "未命名" }}</td><td>{{ planFor(item)?.name || "-" }} · {{ planFor(item)?.cpu || "-" }}C / {{ planFor(item)?.memoryGb || "-" }}GB</td><td>{{ formatUsdMicros(item.chargeUsdMicros) }}/月</td><td>{{ formatDate(item.createdAt) }}</td><td>{{ formatDate(item.paidThrough) }}</td><td><span class="status-pill" :class="{ good: resourceStatusLabel(item) === '可用' }">{{ resourceStatusLabel(item) }}</span></td></tr><tr v-if="!computes.length"><td colspan="6" class="empty-cell">暂无计算资源</td></tr></tbody></table></div></section>
          </section>

          <section v-else-if="path.startsWith('/console/storage')" class="resource-page">
            <div class="page-actions"><div><h2>存储资源</h2><p>按月开通，可挂载到计算资源。</p></div><button class="button primary" type="button" :disabled="!computes.length || !plans.length" @click="openModal('storage')"><Plus :size="16" />开通存储</button></div>
            <div class="plan-grid"><article v-for="plan in plans" :key="plan.id" class="plan-panel"><span class="plan-name">{{ plan.diskGb }}GB</span><strong>{{ formatUsdMicros(storageMonthlyPrice(storageQuotes, plan.id)) }}<small>/月</small></strong><p>{{ plan.name }} 存储规格</p><button class="button secondary wide" type="button" :disabled="!computes.length" @click="storageForm.packageId = plan.id; openModal('storage')">选择 {{ plan.diskGb }}GB</button></article></div>
            <section class="panel"><div class="panel-title"><h2>我的存储资源</h2></div><div class="table-wrap"><table><thead><tr><th>名称</th><th>容量</th><th>价格</th><th>购买时间</th><th>到期时间</th><th>挂载</th><th>状态</th></tr></thead><tbody><tr v-for="item in storages" :key="item.id"><td>{{ item.name || "未命名" }}</td><td>{{ item.sizeGb }}GB</td><td>{{ formatUsdMicros(item.chargeUsdMicros) }}/月</td><td>{{ formatDate(item.createdAt) }}</td><td>{{ formatDate(item.paidThrough) }}</td><td><span v-if="attachmentFor(item)">{{ attachmentFor(item).mountPath || "-" }} · 已挂载</span><button v-else class="text-button" type="button" :disabled="resourceStatusLabel(item) !== '可用' || mutationBusy" @click="mountStorage(item)">挂载</button></td><td><span class="status-pill" :class="{ good: resourceStatusLabel(item) === '可用' }">{{ resourceStatusLabel(item) }}</span></td></tr><tr v-if="!storages.length"><td colspan="7" class="empty-cell">暂无存储资源</td></tr></tbody></table></div></section>
          </section>

          <section v-else-if="path.startsWith('/console/gateway')" class="gateway-page">
            <section class="gateway-summary panel"><div><WalletCards /><span>可用余额<strong>{{ formatAvailableBalance(balance) }}</strong></span></div><div><ShieldCheck /><span>API 调用<strong :class="gatewayHealthy ? 'positive' : ''">{{ errors.gateway ? "暂不可用" : gatewayHealthy ? "正常" : "需要处理" }}</strong></span></div><div><CalendarDays /><span>最近使用<strong>{{ formatDate(gatewayUsage.lastUsedAt, true) }}</strong></span></div></section>
            <div v-if="errors.gateway" class="inline-error"><AlertCircle :size="17" />{{ errors.gateway }}<button type="button" @click="loadGateway">重试</button></div>
            <section class="panel gateway-detail">
              <div class="tabs" role="tablist"><button v-for="tab in [{id:'overview',label:'概览'},{id:'usage',label:'用量'},{id:'key',label:'API Key'}]" :key="tab.id" type="button" role="tab" :aria-selected="gatewayTab === tab.id" :class="{ active: gatewayTab === tab.id }" @click="gatewayTab = tab.id">{{ tab.label }}</button></div>
              <div v-if="gatewayTab === 'overview' || gatewayTab === 'usage'" class="data-section"><h2>用量</h2><dl class="data-list"><div><dt>近 5 小时</dt><dd>{{ formatUsdMicros(gatewayUsage.usage5hUsdMicros) }}</dd></div><div><dt>近 1 天</dt><dd>{{ formatUsdMicros(gatewayUsage.usage1dUsdMicros) }}</dd></div><div><dt>近 7 天</dt><dd>{{ formatUsdMicros(gatewayUsage.usage7dUsdMicros) }}</dd></div><div><dt>累计已用</dt><dd>{{ formatUsdMicros(gatewayUsage.quotaUsedUsdMicros) }}</dd></div><div><dt>最近使用</dt><dd>{{ formatDate(gatewayUsage.lastUsedAt, true) }}</dd></div></dl></div>
              <div v-if="gateway && (gatewayTab === 'overview' || gatewayTab === 'key')" class="data-section"><h2>API Key</h2><div class="key-row"><span><KeyRound :size="17" />{{ gatewayKey.name }}</span><code>{{ gatewayKey.revealed ? gatewayKey.value : gatewayKey.maskedValue }}</code><span class="status-pill" :class="{ good: gatewayHealthy }">{{ gatewayHealthy ? "可用" : "需要处理" }}</span><div class="key-actions"><button v-if="!gatewayKey.revealed" class="text-button" type="button" :disabled="gatewayBusy" @click="revealGatewayKey"><Eye :size="15" />显示</button><button v-else class="text-button" type="button" @click="hideGatewayKey"><EyeOff :size="15" />隐藏</button><button class="text-button" type="button" :disabled="!gatewayKey.value" @click="copyGatewayKey"><Copy :size="15" />复制</button></div></div></div>
              <div v-else-if="gatewayTab === 'overview' || gatewayTab === 'key'" class="data-section"><h2>API Key</h2><p class="empty-copy">暂不可用</p></div>
              <div v-if="gatewayTab === 'overview'" class="data-section"><h2>相关资源</h2><dl class="data-list"><div><dt>Workspace</dt><dd>{{ workspace?.name || "-" }}</dd></div><div><dt>计算</dt><dd>{{ currentCompute?.name || "-" }}<template v-if="currentCompute"> · {{ planFor(currentCompute)?.name || "-" }}</template></dd></div><div><dt>存储</dt><dd>{{ currentStorage?.name || "-" }}<template v-if="currentStorage"> · {{ currentStorage.sizeGb }}GB</template></dd></div><div><dt>挂载</dt><dd>{{ currentAttachment ? `${currentAttachment.mountPath || "-"} · 已挂载` : "-" }}</dd></div></dl></div>
            </section>
          </section>

          <section v-else-if="path.startsWith('/console/billing')" class="billing-page">
            <div class="metric-row billing-metrics"><article><CircleDollarSign /><span>可用余额<strong>{{ formatAvailableBalance(balance) }}</strong></span></article><article><WalletCards /><span>当前固定月支出<strong>{{ formatUsdMicros(currentFixedSpend) }}</strong></span></article><article><Activity /><span>近 7 天 AI 用量<strong>{{ errors.gateway ? "暂不可用" : formatUsdMicros(gatewayUsage.usage7dUsdMicros) }}</strong></span></article></div>
            <section class="panel"><div class="panel-title"><h2>当前资源</h2></div><div class="table-wrap"><table><thead><tr><th>资源</th><th>价格</th><th>购买时间</th><th>有效期至</th><th>续费</th><th>状态</th></tr></thead><tbody><tr v-for="item in allBillableResources" :key="item.id"><td>{{ resourceTypeLabel(item.resourceType || (computes.includes(item) ? "compute" : "storage")) }} · {{ item.name || "未命名" }}</td><td>{{ formatUsdMicros(item.chargeUsdMicros) }}/月</td><td>{{ formatDate(item.createdAt) }}</td><td>{{ formatDate(item.paidThrough) }}</td><td>{{ item.autoRenew === true ? "自动续费" : item.autoRenew === false ? "手动续费" : "-" }}</td><td><span class="status-pill" :class="{ good: item.billingStatus === 'active' }">{{ customerBillingStatusLabel(item.billingStatus) }}</span></td></tr><tr v-if="!allBillableResources.length"><td colspan="6" class="empty-cell">暂无资源</td></tr></tbody></table></div></section>
            <section class="panel"><div class="panel-title"><h2>交易记录</h2></div><div v-if="errors.receipts" class="inline-error"><AlertCircle :size="17" />{{ errors.receipts }}<button type="button" @click="loadReceipts()">重试</button></div><div class="table-wrap"><table><thead><tr><th>时间</th><th>交易</th><th>资源</th><th>金额</th><th>有效期至</th><th>状态</th></tr></thead><tbody><tr v-for="receipt in receipts" :key="receipt.receiptId"><td>{{ formatDate(receipt.createdAt, true) }}</td><td>{{ receiptTitle(receipt) }}</td><td>{{ resourceTypeLabel(receipt.resourceType) }}</td><td>{{ formatUsdMicros(receipt.chargeUsdMicros) }}</td><td>{{ formatDate(receipt.paidThrough) }}</td><td>{{ receipt.status || "已记录" }}</td></tr><tr v-if="!receipts.length && !loading.receipts"><td colspan="6" class="empty-cell">暂无交易记录</td></tr></tbody></table></div><div v-if="receiptPage.hasMore" class="load-more"><button class="button secondary" type="button" :disabled="loading.receipts" @click="loadReceipts(false)">加载更多</button></div></section>
          </section>
        </template>
      </div>
    </section>
  </div>

  <div v-if="modal" class="modal-backdrop" role="presentation" @click.self="modal = ''">
    <section class="modal" role="dialog" aria-modal="true" :aria-label="modal">
      <header><h2>{{ modal === "workspace" ? "开通 Workspace" : modal === "compute" ? "开通计算" : modal === "storage" ? "开通存储" : "新建用户" }}</h2><button class="icon-button" type="button" aria-label="关闭" @click="modal = ''"><X :size="18" /></button></header>

      <form v-if="modal === 'workspace'" @submit.prevent="launchWorkspace">
        <label>Workspace 名称<input v-model.trim="launchForm.workspaceName" required maxlength="80" placeholder="例如：蛋白质组学研究" /></label>
        <label>计算资源名称<input v-model.trim="launchForm.computeName" required maxlength="80" placeholder="例如：蛋白分析节点" /></label>
        <label>存储资源名称<input v-model.trim="launchForm.storageName" required maxlength="80" placeholder="例如：实验数据盘" /></label>
        <fieldset><legend>规格</legend><label v-for="plan in plans" :key="plan.id" class="plan-option" :class="{ selected: launchForm.packageId === plan.id }"><input v-model="launchForm.packageId" type="radio" :value="plan.id" /><span><strong>{{ plan.name }}</strong><small>{{ plan.cpu }}C / {{ plan.memoryGb }}GB · {{ plan.diskGb }}GB 存储</small></span><b>{{ formatUsdMicros(workspaceMonthlyPrice(plan, storageQuotes)) }}/月</b></label></fieldset>
        <p v-if="launchStatus" class="inline-notice">{{ launchStatus }}</p>
        <footer><button class="button secondary" type="button" @click="modal = ''">取消</button><button class="button primary" type="submit" :disabled="mutationBusy">{{ mutationBusy ? "处理中..." : "确认开通" }}</button></footer>
      </form>

      <form v-else-if="modal === 'compute'" @submit.prevent="buyCompute">
        <label>计算资源名称<input v-model.trim="computeForm.name" required maxlength="80" placeholder="例如：蛋白分析节点" /></label>
        <fieldset><legend>规格</legend><label v-for="plan in plans" :key="plan.id" class="plan-option" :class="{ selected: computeForm.packageId === plan.id }"><input v-model="computeForm.packageId" type="radio" :value="plan.id" /><span><strong>{{ plan.name }}</strong><small>{{ plan.cpu }}C / {{ plan.memoryGb }}GB</small></span><b>{{ formatUsdMicros(plan.price?.chargeUsdMicros) }}/月</b></label></fieldset>
        <footer><button class="button secondary" type="button" @click="modal = ''">取消</button><button class="button primary" type="submit" :disabled="mutationBusy">{{ mutationBusy ? "处理中..." : "确认开通" }}</button></footer>
      </form>

      <form v-else-if="modal === 'storage'" @submit.prevent="buyStorage">
        <label>存储资源名称<input v-model.trim="storageForm.name" required maxlength="80" placeholder="例如：实验数据盘" /></label>
        <fieldset><legend>容量</legend><label v-for="plan in plans" :key="plan.id" class="plan-option" :class="{ selected: storageForm.packageId === plan.id }"><input v-model="storageForm.packageId" type="radio" :value="plan.id" /><span><strong>{{ plan.diskGb }}GB</strong><small>{{ plan.name }}</small></span><b>{{ formatUsdMicros(storageMonthlyPrice(storageQuotes, plan.id)) }}/月</b></label></fieldset>
        <label>关联计算资源<select v-model="storageForm.computeAllocationId" required><option value="" disabled>请选择</option><option v-for="item in computes" :key="item.id" :value="item.id">{{ item.name || "未命名计算" }}</option></select></label>
        <footer><button class="button secondary" type="button" @click="modal = ''">取消</button><button class="button primary" type="submit" :disabled="mutationBusy">{{ mutationBusy ? "处理中..." : "确认开通" }}</button></footer>
      </form>

      <form v-else @submit.prevent="createCustomerUser">
        <label>登录邮箱<input v-model.trim="adminUserForm.email" type="email" required /></label>
        <label>初始密码<input v-model="adminUserForm.password" type="password" required minlength="12" /></label>
        <label>姓名<input v-model.trim="adminUserForm.name" /></label>
        <label>账号 ID<input v-model.trim="adminUserForm.accountId" required /></label>
        <label>Gateway 用户 ID<input v-model.trim="adminUserForm.sub2apiUserId" type="number" min="1" step="1" required /></label>
        <footer><button class="button secondary" type="button" @click="modal = ''">取消</button><button class="button primary" type="submit" :disabled="mutationBusy">{{ mutationBusy ? "创建中..." : "创建用户" }}</button></footer>
      </form>
    </section>
  </div>

  <div v-if="toast.text" class="toast" :class="toast.tone" role="status">{{ toast.text }}</div>
</template>
