import type { Gizmo, NetworkDevice, Vlan } from "./api"
import { vlanColors } from "./vlan-colors.ts"
import { contains, type FilterField } from "./list-controls.ts"

export const vlanFilters = {
  vlan_id: (v: Vlan, value: string) => String(v.vlan_id) === value,
  name: (v: Vlan, value: string) => contains(v.name, value),
  description: (v: Vlan, value: string) => contains(v.description, value),
  color: (v: Vlan, value: string) => (v.color ?? "default") === value,
}
export const vlanSortValues = {
  vlan_id: (v: Vlan) => v.vlan_id,
  name: (v: Vlan) => v.name,
  description: (v: Vlan) => v.description,
}
export function vlanFilterFields(items: readonly Vlan[]): FilterField[] {
  return [
    { key: "vlan_id", label: "VLAN ID", options: vlanOptions(items) },
    { key: "name", label: "Name" },
    { key: "description", label: "Description" },
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
export const deviceFilters = {
  display_name: (d: NetworkDevice, value: string) =>
    contains(d.display_name, value),
  mac_address: (d: NetworkDevice, value: string) =>
    contains(d.mac_address, value),
  vlan_id: (d: NetworkDevice, value: string) =>
    String(d.vlan.vlan_id) === value,
  status: (d: NetworkDevice, value: string) =>
    (d.disabled ? "disabled" : "enabled") === value,
}
export const deviceSortValues = {
  display_name: (d: NetworkDevice) => d.display_name,
  mac_address: (d: NetworkDevice) => d.mac_address,
  vlan: (d: NetworkDevice) => d.vlan.name,
  status: (d: NetworkDevice) => (d.disabled ? "Disabled" : "Enabled"),
}
export function deviceFilterFields(vlans: readonly Vlan[]): FilterField[] {
  return [
    { key: "display_name", label: "Name" },
    { key: "mac_address", label: "MAC" },
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
export const gizmoFilters = {
  display_name: (g: Gizmo, value: string) =>
    contains(g.network_device.display_name, value),
  mac_address: (g: Gizmo, value: string) =>
    contains(g.network_device.mac_address, value),
  kiosk_url: (g: Gizmo, value: string) => contains(g.kiosk_url ?? "", value),
  destination: (g: Gizmo, value: string) =>
    (g.kiosk_url ? "configured" : "unconfigured") === value,
}
export const gizmoSortValues = {
  display_name: (g: Gizmo) => g.network_device.display_name,
  mac_address: (g: Gizmo) => g.network_device.mac_address,
  destination: (g: Gizmo) => g.kiosk_url,
}
export function gizmoFilterFields(): FilterField[] {
  return [
    { key: "display_name", label: "Name" },
    { key: "mac_address", label: "MAC" },
    { key: "kiosk_url", label: "URL" },
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
