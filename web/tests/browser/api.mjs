import { createServer } from "node:http"
const time = "2025-01-01T00:00:00Z"
const vlan = {
  vlan_id: 100,
  name: "Private",
  description: "",
  color: "blue",
  created_at: time,
  updated_at: time,
}
const device = {
  id: "00000000-0000-4000-8000-000000000001",
  display_name: "Example laptop",
  mac_address: "02:00:00:00:00:01",
  disabled: false,
  private_wifi_password_enabled: true,
  wifi_password_set: true,
  vlan,
  created_at: time,
  updated_at: time,
}
const sessions = ["First client", "Second client"].map(
  (client_name, index) => ({
    id: "session-" + index,
    username: "tester",
    client_name,
    is_current: false,
    created_at: time,
    expires_at: "2030-01-01T00:00:00Z",
  })
)
createServer((req, res) => {
  const url = new URL(req.url, "http://127.0.0.1")
  if (req.method !== "GET") {
    res.writeHead(405)
    return res.end()
  }
  const routes = {
    "/healthz": { status: "ok" },
    "/api/v1/profile": {
      username: "tester",
      name: "Test user",
      email: "tester@example.test",
      groups: ["Admins"],
      is_admin: true,
    },
    "/api/v1/networks/devices": { items: [device] },
    "/api/v1/networks/vlans": { items: [vlan] },
    "/api/v1/gizmos": {
      items: [
        {
          network_device: device,
          kiosk_url: "https://example.test",
          created_at: time,
          updated_at: time,
        },
      ],
    },
    "/api/v1/preferences": {},
    "/api/v1/profile/sessions": {
      items: sessions,
      pagination: { limit: 25, offset: 0, total: 2 },
    },
    "/api/v1/admin/auth/sessions": {
      items: sessions,
      pagination: { limit: 25, offset: 0, total: 2 },
    },
    "/api/v1/admin/audit_logs": {
      items: [
        {
          id: "audit-1",
          actor_username: "tester",
          action: "update",
          resource_type: "device",
          resource_id: device.id,
          metadata: { example: true },
          created_at: time,
        },
      ],
      filter_options: { actions: ["update"], resource_types: ["device"] },
      pagination: { limit: 25, offset: 0, total: 100 },
    },
  }
  const body = routes[url.pathname]
  setTimeout(
    () => {
      res.writeHead(body ? 200 : 404, { "content-type": "application/json" })
      res.end(JSON.stringify(body ?? { error: "Unknown fixture" }))
    },
    url.pathname.includes("audit_logs") ? 200 : 0
  )
}).listen(4301, "127.0.0.1")
