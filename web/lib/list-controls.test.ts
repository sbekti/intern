import assert from "node:assert/strict"
import test from "node:test"
import {
  filterAndSort,
  readFilters,
  readSort,
  updateListQuery,
} from "./list-controls.ts"
import {
  deviceFilterValues,
  deviceSortValues,
  gizmoFilterValues,
  gizmoSortValues,
  vlanFilterValues,
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

test("filters are exact, case sensitive AND conditions without changing source records", () => {
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
  for (const name of ["camera", "Cam"]) {
    assert.equal(
      filterAndSort(
        devices,
        { display_name: name },
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
