import type { Gizmo, NetworkDevice, Vlan } from "./api"
import { vlanColors } from "./vlan-colors.ts"
import { filterOptions, type FilterField } from "./list-controls.ts"

export const vlanFilterValues = {
  vlan_id: (v: Vlan) => String(v.vlan_id),
  name: (v: Vlan) => v.name,
  color: (v: Vlan) => v.color ?? "default",
}
export const vlanSortValues = {
  vlan_id: (v: Vlan) => v.vlan_id,
  name: (v: Vlan) => v.name,
  description: (v: Vlan) => v.description,
}
export function vlanFilterFields(items: readonly Vlan[]): FilterField[] {
  return [
    { key: "vlan_id", label: "VLAN ID", options: vlanOptions(items) },
    {
      key: "name",
      label: "Name",
      options: filterOptions(items.map((v) => v.name)),
    },
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
export const deviceFilterValues = {
  display_name: (d: NetworkDevice) => d.display_name,
  mac_address: (d: NetworkDevice) => d.mac_address,
  vlan_id: (d: NetworkDevice) => String(d.vlan.vlan_id),
  status: (d: NetworkDevice) => (d.disabled ? "disabled" : "enabled"),
}
export const deviceSortValues = {
  display_name: (d: NetworkDevice) => d.display_name,
  mac_address: (d: NetworkDevice) => d.mac_address,
  vlan: (d: NetworkDevice) => d.vlan.name,
  status: (d: NetworkDevice) => (d.disabled ? "Disabled" : "Enabled"),
}
export function deviceFilterFields(
  items: readonly NetworkDevice[],
  vlans: readonly Vlan[]
): FilterField[] {
  return [
    {
      key: "display_name",
      label: "Name",
      options: filterOptions(items.map((d) => d.display_name)),
    },
    {
      key: "mac_address",
      label: "MAC",
      options: filterOptions(items.map((d) => d.mac_address)),
    },
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
export const gizmoFilterValues = {
  display_name: (g: Gizmo) => g.network_device.display_name,
  mac_address: (g: Gizmo) => g.network_device.mac_address,
  destination: (g: Gizmo) => (g.kiosk_url ? "configured" : "unconfigured"),
}
export const gizmoSortValues = {
  display_name: (g: Gizmo) => g.network_device.display_name,
  mac_address: (g: Gizmo) => g.network_device.mac_address,
  destination: (g: Gizmo) => g.kiosk_url,
}
export function gizmoFilterFields(items: readonly Gizmo[]): FilterField[] {
  return [
    {
      key: "display_name",
      label: "Name",
      options: filterOptions(items.map((g) => g.network_device.display_name)),
    },
    {
      key: "mac_address",
      label: "MAC",
      options: filterOptions(items.map((g) => g.network_device.mac_address)),
    },
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
