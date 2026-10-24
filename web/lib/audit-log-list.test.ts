import assert from "node:assert/strict"
import test from "node:test"
import { auditContextFields, auditFilterFields } from "./audit-log-list.ts"

test("audit categories use all server options, independently of the displayed rows", () => {
  assert.deepEqual(
    auditFilterFields({
      actions: ["vlan.create", "device.update", "device.update"],
      resource_types: ["vlan", "network_device"],
    }),
    [
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
    ]
  )
  assert.deepEqual(
    auditContextFields.map(({ key }) => key),
    ["resource_id", "actor_username"]
  )
})
