export const vlanColors = [
  { value: "default", label: "Default" },
  { value: "blue", label: "Blue" },
  { value: "amber", label: "Amber" },
  { value: "violet", label: "Violet" },
  { value: "rose", label: "Rose" },
  { value: "orange", label: "Orange" },
  { value: "cyan", label: "Cyan" },
  { value: "slate", label: "Slate" },
  { value: "green", label: "Green" },
] as const

export type VlanColor = (typeof vlanColors)[number]["value"]
