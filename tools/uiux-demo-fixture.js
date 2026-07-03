export const uiuxDemoAccounts = Object.freeze([
  {
    label: "Lab Owner",
    id: "usr-owner-uiux",
    email: "owner@opl.local",
    password: "OplOwnerPass2026!",
    name: "Lab Owner",
    role: "pi",
    accountId: "acct-owner-uiux"
  },
  {
    label: "Admin",
    id: "usr-admin-uiux",
    email: "admin@opl.local",
    password: "OplAdminPass2026!",
    name: "OPL Admin",
    role: "admin",
    accountId: "admin"
  }
]);

export function uiuxDemoAuthSeedJson() {
  return JSON.stringify(uiuxDemoAccounts.map(({ label, ...account }) => account));
}
