import type { Gizmo, NetworkDevice, Vlan } from "./api"
import { vlanColors } from "./vlan-colors.ts"
import type { FilterField } from "./list-controls.ts"

export const vlanSearchValues = (v: Vlan) => [
  String(v.vlan_id),
  v.name,
  v.description,
]
export const vlanFilterValues = {
  color: (v: Vlan) => v.color ?? "default",
}
export const vlanSortValues = {
  vlan_id: (v: Vlan) => v.vlan_id,
  name: (v: Vlan) => v.name,
  description: (v: Vlan) => v.description,
}
export function vlanFilterFields(): FilterField[] {
  return [
    {
      key: "color",
      label: "Color",
      options: vlanColors.map(({ value, label }) => ({ value, label })),
    },
  ]
}

function vlanOptions(items: readonly Vlan[]) {
  return [...items]
    .sort((a, b) => a.vlan_id - b.vlan_id)
    .map((v) => ({
      value: String(v.vlan_id),
      label: `${v.name} (${v.vlan_id})`,
    }))
}
export const deviceSearchValues = (d: NetworkDevice) => [
  d.display_name,
  d.mac_address,
  d.vlan.name,
  String(d.vlan.vlan_id),
]
export const deviceFilterValues = {
  vlan_id: (d: NetworkDevice) => String(d.vlan.vlan_id),
  status: (d: NetworkDevice) => (d.disabled ? "disabled" : "enabled"),
}
export const deviceSortValues = {
  display_name: (d: NetworkDevice) => d.display_name,
  mac_address: (d: NetworkDevice) => d.mac_address,
  vlan: (d: NetworkDevice) => d.vlan.name,
  status: (d: NetworkDevice) => (d.disabled ? "Disabled" : "Enabled"),
}
export function deviceFilterFields(vlans: readonly Vlan[]): FilterField[] {
  return [
    { key: "vlan_id", label: "VLAN", options: vlanOptions(vlans) },
    {
      key: "status",
      label: "Status",
      options: [
        { value: "enabled", label: "Enabled" },
        { value: "disabled", label: "Disabled" },
      ],
    },
  ]
}
export const gizmoSearchValues = (g: Gizmo) => [
  g.network_device.display_name,
  g.network_device.mac_address,
  g.kiosk_url ?? "",
]
export const gizmoFilterValues = {
  destination: (g: Gizmo) => (g.kiosk_url ? "configured" : "unconfigured"),
}
export const gizmoSortValues = {
  display_name: (g: Gizmo) => g.network_device.display_name,
  mac_address: (g: Gizmo) => g.network_device.mac_address,
  destination: (g: Gizmo) => g.kiosk_url,
}
export function gizmoFilterFields(): FilterField[] {
  return [
    {
      key: "destination",
      label: "Destination",
      options: [
        { value: "configured", label: "Configured" },
        { value: "unconfigured", label: "Unconfigured" },
      ],
    },
  ]
}
