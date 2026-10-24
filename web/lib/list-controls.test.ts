import assert from "node:assert/strict"
import test from "node:test"
import {
  filterAndSort,
  readFilters,
  readSort,
  searchItems,
  updateListQuery,
} from "./list-controls.ts"
import {
  deviceFilterValues,
  deviceSortValues,
  deviceSearchValues,
  gizmoFilterValues,
  gizmoSortValues,
  gizmoSearchValues,
  vlanFilterValues,
  vlanSortValues,
  vlanSearchValues,
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
    deviceFilterValues,
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
        deviceFilterValues,
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

test("inventory search matches partial text regardless of case and combines with categories", () => {
  const before = structuredClone(devices)
  for (const query of ["cam", "CAM", " camera "])
    assert.deepEqual(
      searchItems(devices, query, deviceSearchValues).map((d) => d.id),
      ["b", "a"]
    )
  for (const query of ["00:02", "IoT", "20"])
    assert.ok(
      searchItems(devices, query, deviceSearchValues).some((d) => d.id === "b")
    )
  const rows = filterAndSort(
    searchItems(devices, "am", deviceSearchValues),
    { vlan_id: "20", status: "enabled" },
    deviceFilterValues,
    { key: "display_name", direction: "asc" },
    deviceSortValues,
    (d) => d.id
  )
  assert.deepEqual(
    rows.map((d) => d.id),
    ["b"]
  )
  assert.deepEqual(devices, before)
  assert.equal(searchItems(devices, "  ", deviceSearchValues).length, 3)
  assert.equal(searchItems(devices, "missing", deviceSearchValues).length, 0)
})

test("VLAN search includes IDs and descriptions and treats punctuation literally", () => {
  const items = [{ ...vlans[0], description: "Office_LAN 100%" }, vlans[1]]
  for (const query of ["20", "oT", "office", "_", "%"])
    assert.deepEqual(
      searchItems(items, query, vlanSearchValues).map((v) => v.vlan_id),
      [20]
    )
  assert.equal(searchItems(items, "Office LAN", vlanSearchValues).length, 0)
})

test("Gizmo search includes names, MACs, and destination URLs", () => {
  const items: Gizmo[] = devices.map((d, i) => ({
    network_device: {
      ...d,
      display_name: i === 0 ? "jfk1-av-gzm-x1" : d.display_name,
    },
    kiosk_url: i === 0 ? "https://ha.example.test/gizmo-home?kiosk" : null,
    created_at: stamp,
    updated_at: stamp,
  }))
  for (const query of ["gZm", "00:02", "HA.EXAMPLE", "?kiosk"])
    assert.deepEqual(
      searchItems(items, query, gizmoSearchValues).map(
        (g) => g.network_device.id
      ),
      ["b"]
    )
})

test("sorts by displayed values with stable ID ties in both directions", () => {
  for (const direction of ["asc", "desc"] as const) {
    const rows = filterAndSort(
      devices,
      {},
      deviceFilterValues,
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
    deviceFilterValues,
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
      vlanFilterValues,
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
      vlanFilterValues,
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
      gizmoFilterValues,
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
      gizmoFilterValues,
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
