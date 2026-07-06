import React from "react";
import { AccountPage } from "../pages/account/AccountPage.jsx";
import {
  AdminBillingPage,
  AdminCleanupPage,
  AdminDiagnosticsPage,
  AdminE2EPage,
  AdminFabricPage,
  AdminLedgerPage,
  AdminOverviewPage,
  AdminRuntimePage,
  AdminSupportPage,
  AdminUsersPage
} from "../pages/admin/AdminOverviewPage.jsx";
import { BillingPage } from "../pages/billing/BillingPage.jsx";
import { AlertsPage, ApprovalsPage, ReceiptsPage, ResourcesPage } from "../pages/catalog/FabricPages.jsx";
import { GatewayPage } from "../pages/gateway/GatewayPage.jsx";
import { OverviewPage } from "../pages/OverviewPage.jsx";
import {
  ComputeAllocationDetailPage,
  ComputeAllocationsPage,
  CreateComputeAllocationPage,
  CreateStorageAttachmentPage,
  CreateStorageVolumePage,
  ResourceRelationshipPage,
  StorageAttachmentDetailPage,
  StorageAttachmentsPage,
  StorageVolumeDetailPage,
  StorageVolumesPage
} from "../pages/resources/ResourceProvisioningPages.jsx";
import { ForbiddenPage } from "../pages/shared/page-widgets.jsx";
import { NewSupportTicketPage, SupportPage, SupportTicketPage } from "../pages/support/SupportPage.jsx";
import { CreateWorkspacePage } from "../pages/workspaces/CreateWorkspacePage.jsx";
import { WorkspaceDetailPage } from "../pages/workspaces/WorkspaceDetailPage.jsx";
import { WorkspacesPage } from "../pages/workspaces/WorkspacesPage.jsx";

export function renderConsoleRoute(ctx) {
  const { path, route, isAdmin } = ctx;
  if (route.area === "admin" && !isAdmin) return <ForbiddenPage />;
  if (path.startsWith("/admin/users")) return <AdminUsersPage {...ctx} />;
  if (path.startsWith("/admin/billing")) return <AdminBillingPage {...ctx} />;
  if (path.startsWith("/admin/fabric")) return <AdminFabricPage {...ctx} />;
  if (path.startsWith("/admin/ledger")) return <AdminLedgerPage {...ctx} />;
  if (path.startsWith("/admin/runtime")) return <AdminRuntimePage {...ctx} />;
  if (path.startsWith("/admin/diagnostics")) return <AdminDiagnosticsPage {...ctx} />;
  if (path.startsWith("/admin/e2e")) return <AdminE2EPage {...ctx} />;
  if (path.startsWith("/admin/cleanup")) return <AdminCleanupPage {...ctx} />;
  if (path.startsWith("/admin/support")) return <AdminSupportPage {...ctx} />;
  if (path.startsWith("/admin")) return <AdminOverviewPage {...ctx} />;
  if (path.startsWith("/console/compute/new")) return <CreateComputeAllocationPage {...ctx} />;
  if (path.startsWith("/console/compute/")) return <ComputeAllocationDetailPage {...ctx} />;
  if (path.startsWith("/console/compute")) return <ComputeAllocationsPage {...ctx} />;
  if (path.startsWith("/console/storage/new")) return <CreateStorageVolumePage {...ctx} />;
  if (path.startsWith("/console/storage/")) return <StorageVolumeDetailPage {...ctx} />;
  if (path.startsWith("/console/storage")) return <StorageVolumesPage {...ctx} />;
  if (path.startsWith("/console/attachments/new")) return <CreateStorageAttachmentPage {...ctx} />;
  if (path.startsWith("/console/attachments/")) return <StorageAttachmentDetailPage {...ctx} />;
  if (path.startsWith("/console/attachments")) return <StorageAttachmentsPage {...ctx} />;
  if (path.startsWith("/console/resources/relationships")) return <ResourceRelationshipPage {...ctx} />;
  if (path.startsWith("/console/workspaces/new")) return <CreateWorkspacePage {...ctx} />;
  if (path.startsWith("/console/workspaces/") || path.startsWith("/admin/workspaces/")) return <WorkspaceDetailPage {...ctx} />;
  if (path.startsWith("/console/workspaces")) return <WorkspacesPage {...ctx} />;
  if (path.startsWith("/console/gateway")) return <GatewayPage {...ctx} />;
  if (path.startsWith("/console/billing")) return <BillingPage {...ctx} />;
  if (path.startsWith("/console/account")) return <AccountPage {...ctx} />;
  if (path.startsWith("/console/support/new")) return <NewSupportTicketPage {...ctx} />;
  if (path.startsWith("/console/support/")) return <SupportTicketPage {...ctx} />;
  if (path.startsWith("/console/support")) return <SupportPage {...ctx} />;
  if (path.startsWith("/console/resources")) return <ResourcesPage {...ctx} />;
  if (path.startsWith("/console/approvals")) return <ApprovalsPage {...ctx} />;
  if (path.startsWith("/console/receipts")) return <ReceiptsPage {...ctx} />;
  if (path.startsWith("/console/alerts")) return <AlertsPage {...ctx} />;
  if (path === "/403") return <ForbiddenPage />;
  return <OverviewPage {...ctx} />;
}
