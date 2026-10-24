import assert from "node:assert/strict"
import test from "node:test"
import { auditFilterFields, auditFilterKeys } from "./audit-log-list.ts"

const options = {
  actions: ["vlan.create", "device.update", "device.update"],
  resource_types: ["vlan", "network_device"],
}

test("audit fields offer contains text and complete, exact category choices", () => {
  assert.deepEqual(auditFilterFields(options, {}), [
    { key: "actor_username_contains", label: "Actor" },
    {
      key: "action",
      label: "Action",
      options: [
        { value: "device.update", label: "device.update" },
        { value: "vlan.create", label: "vlan.create" },
      ],
    },
    {
      key: "resource_type",
      label: "Resource Type",
      options: [
        { value: "network_device", label: "network_device" },
        { value: "vlan", label: "vlan" },
      ],
    },
    { key: "resource_id", label: "Resource ID", exact: true },
    { key: "metadata_contains", label: "Metadata" },
  ])
})

test("exact actor restrictions from URLs remain visible without adding another search mode", () => {
  const fields = auditFilterFields(options, { actor_username: "Alice" })
  assert.deepEqual(fields.at(-1), {
    key: "actor_username",
    label: "Actor (exact)",
    exact: true,
  })
  assert.deepEqual(
    fields.map(({ key }) => key),
    [...auditFilterKeys]
  )
})
