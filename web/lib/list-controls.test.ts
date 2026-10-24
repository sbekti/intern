import assert from "node:assert/strict"
import test from "node:test"
import {
  filterAndSort,
  readFilters,
  readSort,
  contains,
  updateListQuery,
} from "./list-controls.ts"
import {
  deviceFilters,
  deviceSortValues,
  gizmoFilters,
  gizmoSortValues,
  vlanFilters,
  vlanSortValues,
} from "./network-list.ts"
import type { Gizmo, NetworkDevice, Vlan } from "./api"

const stamp = "2026-10-07T00:00:00Z"
const vlans: Vlan[] = [
  {
    vlan_id: 20,
    name: "iot",
    color: "violet",
    description: "IoT",
    created_at: stamp,
    updated_at: stamp,
  },
  {
    vlan_id: 10,
    name: "guest",
    description: "Guest",
    created_at: stamp,
    updated_at: stamp,
  },
]
const devices: NetworkDevice[] = [
  {
    id: "b",
    display_name: "Camera",
    mac_address: "00:00:00:00:00:02",
    disabled: false,
    vlan: vlans[0],
    created_at: stamp,
    updated_at: stamp,
  },
  {
    id: "a",
    display_name: "Camera",
    mac_address: "00:00:00:00:00:01",
    disabled: true,
    vlan: vlans[0],
    created_at: stamp,
    updated_at: stamp,
  },
  {
    id: "c",
    display_name: "Guest",
    mac_address: "00:00:00:00:00:03",
    disabled: false,
    vlan: vlans[1],
    created_at: stamp,
    updated_at: stamp,
  },
]

test("category filters remain exact AND conditions without changing source records", () => {
  const before = structuredClone(devices)
  const rows = filterAndSort(
    devices,
    { vlan_id: "20", status: "enabled" },
    deviceFilters,
    { key: "display_name", direction: "asc" },
    deviceSortValues,
    (d) => d.id
  )
  assert.deepEqual(
    rows.map((d) => d.id),
    ["b"]
  )
  for (const status of ["Enabled", "enab"]) {
    assert.equal(
      filterAndSort(
        devices,
        { status },
        deviceFilters,
        { key: "display_name", direction: "asc" },
        deviceSortValues,
        (d) => d.id
      ).length,
      0
    )
  }
  assert.deepEqual(devices, before)
  assert.equal(rows[0], devices[0])
})

test("contains matching is literal, trimmed, and case insensitive", () => {
  for (const value of ["cam", "CAM", " camera "])
    assert.ok(contains("Camera", value))
  assert.ok(contains("Office_LAN 100%", "_"))
  assert.ok(contains("Office_LAN 100%", "%"))
  assert.equal(contains("Office_LAN", "Office LAN"), false)
})

test("text tokens search their selected field and combine with exact categories", () => {
  const before = structuredClone(devices)
  for (const name of ["cam", "CAM", " camera "])
    assert.deepEqual(
      filterAndSort(
        devices,
        { display_name: name },
        deviceFilters,
        { key: "display_name", direction: "asc" },
        deviceSortValues,
        (d) => d.id
      ).map((d) => d.id),
      ["a", "b"]
    )
  assert.equal(deviceFilters.display_name(devices[0], "iot"), false)
  assert.equal(deviceFilters.mac_address(devices[0], "Camera"), false)
  assert.equal(deviceFilters.mac_address(devices[0], "00:02"), true)
  assert.equal(deviceFilters.vlan_id(devices[0], "2"), false)
  const rows = filterAndSort(
    devices,
    {
      display_name: "am",
      mac_address: "00:02",
      vlan_id: "20",
      status: "enabled",
    },
    deviceFilters,
    { key: "display_name", direction: "asc" },
    deviceSortValues,
    (d) => d.id
  )
  assert.deepEqual(
    rows.map((d) => d.id),
    ["b"]
  )
  assert.deepEqual(devices, before)
})

test("VLAN names and descriptions contain text while IDs and colors remain exact", () => {
  const item = { ...vlans[0], description: "Office_LAN 100%" }
  for (const query of ["office", "_", "%"])
    assert.ok(vlanFilters.description(item, query))
  assert.ok(vlanFilters.name(item, "oT"))
  assert.equal(vlanFilters.name(item, "office"), false)
  assert.equal(vlanFilters.vlan_id(item, "2"), false)
  assert.ok(vlanFilters.vlan_id(item, "20"))
  assert.equal(vlanFilters.color(item, "vio"), false)
})

test("Gizmo name, MAC and URL tokens match independently", () => {
  const item: Gizmo = {
    network_device: { ...devices[0], display_name: "jfk1-av-gzm-x1" },
    kiosk_url: "https://ha.example.test/gizmo-home?kiosk",
    created_at: stamp,
    updated_at: stamp,
  }
  assert.ok(gizmoFilters.display_name(item, "gZm"))
  assert.ok(gizmoFilters.mac_address(item, "00:02"))
  assert.ok(gizmoFilters.kiosk_url(item, "HA.EXAMPLE"))
  assert.ok(gizmoFilters.kiosk_url(item, "?kiosk"))
  assert.equal(gizmoFilters.display_name(item, "HA.EXAMPLE"), false)
  assert.equal(
    gizmoFilters.kiosk_url({ ...item, kiosk_url: null }, "ha"),
    false
  )
  assert.equal(gizmoFilters.destination(item, "config"), false)
})

test("sorts by displayed values with stable ID ties in both directions", () => {
  for (const direction of ["asc", "desc"] as const) {
    const rows = filterAndSort(
      devices,
      {},
      deviceFilters,
      { key: "display_name", direction },
      deviceSortValues,
      (d) => d.id
    )
    assert.deepEqual(
      rows.map((d) => d.id),
      direction === "asc" ? ["a", "b", "c"] : ["c", "a", "b"]
    )
  }
  const rows = filterAndSort(
    devices,
    {},
    deviceFilters,
    { key: "vlan", direction: "asc" },
    deviceSortValues,
    (d) => d.id
  )
  assert.deepEqual(
    rows.map((d) => d.id),
    ["c", "a", "b"]
  )
})

test("VLAN sorting is numeric and old API color omissions mean Default", () => {
  assert.deepEqual(
    filterAndSort(
      vlans,
      {},
      vlanFilters,
      { key: "vlan_id", direction: "asc" },
      vlanSortValues,
      (v) => v.vlan_id
    ).map((v) => v.vlan_id),
    [10, 20]
  )
  assert.deepEqual(
    filterAndSort(
      vlans,
      { color: "default" },
      vlanFilters,
      { key: "name", direction: "asc" },
      vlanSortValues,
      (v) => v.vlan_id
    ).map((v) => v.vlan_id),
    [10]
  )
})

test("Gizmo destination filtering and sorting always leave unconfigured destinations last", () => {
  const items: Gizmo[] = devices.map((d, i) => ({
    network_device: d,
    kiosk_url: i === 0 ? null : `https://example.test/${i}`,
    created_at: stamp,
    updated_at: stamp,
  }))
  for (const direction of ["asc", "desc"] as const) {
    const rows = filterAndSort(
      items,
      {},
      gizmoFilters,
      { key: "destination", direction },
      gizmoSortValues,
      (g) => g.network_device.id
    )
    assert.equal(rows.at(-1)?.kiosk_url, null)
  }
  assert.equal(
    filterAndSort(
      items,
      { destination: "unconfigured" },
      gizmoFilters,
      { key: "display_name", direction: "asc" },
      gizmoSortValues,
      (g) => g.network_device.id
    ).length,
    1
  )
})

test("query changes encode values and preserve page size, sorting and unrelated parameters", () => {
  const query = updateListQuery(
    "limit=100&offset=200&sort_by=action&sort_dir=asc&view=all",
    { actor_username: "Alice & Bob" },
    true
  )
  const params = new URLSearchParams(query)
  assert.equal(params.get("actor_username"), "Alice & Bob")
  assert.equal(params.get("offset"), null)
  assert.equal(params.get("limit"), "100")
  assert.equal(params.get("sort_by"), "action")
  assert.equal(params.get("view"), "all")
  const cleared = new URLSearchParams(
    updateListQuery(query, { actor_username: undefined }, true)
  )
  assert.equal(cleared.get("actor_username"), null)
  assert.equal(cleared.get("sort_dir"), "asc")
})

test("search and category changes preserve each other and reset server pagination", () => {
  const searched = updateListQuery(
    "action=device.update&resource_id=device-4&limit=25&offset=50&sort_by=action&sort_dir=asc",
    { q: "Camera %_" },
    true
  )
  let params = new URLSearchParams(searched)
  assert.equal(params.get("q"), "Camera %_")
  assert.equal(params.get("action"), "device.update")
  assert.equal(params.get("resource_id"), "device-4")
  assert.equal(params.get("offset"), null)
  params = new URLSearchParams(
    updateListQuery(searched, { action: "vlan.create" }, true)
  )
  assert.equal(params.get("q"), "Camera %_")
  params = new URLSearchParams(
    updateListQuery(
      params.toString(),
      { q: undefined, action: undefined, resource_id: undefined },
      true
    )
  )
  assert.deepEqual(Object.fromEntries(params), {
    limit: "25",
    sort_by: "action",
    sort_dir: "asc",
  })
})

test("URL readers trim known filters, retain unavailable values, and safely default invalid sorting", () => {
  const params = new URLSearchParams(
    "name=%20removed%20&ignored=x&sort_by=unknown&sort_dir=asc"
  )
  assert.deepEqual(readFilters(params, ["name", "color"]), { name: "removed" })
  const fallback = { key: "created_at", direction: "desc" } as const
  assert.deepEqual(
    readSort(params, ["created_at", "action"], fallback),
    fallback
  )
  assert.deepEqual(
    readSort(
      new URLSearchParams("sort_by=action&sort_dir=desc"),
      ["created_at", "action"],
      fallback
    ),
    { key: "action", direction: "desc" }
  )
  assert.deepEqual(
    readSort(
      new URLSearchParams("sort_by=action&sort_dir=bad"),
      ["created_at", "action"],
      fallback
    ),
    fallback
  )
})
