import { Badge } from "@/components/ui/badge"
import type { VlanColor } from "@/lib/vlan-colors"

export function VlanBadge({
  name,
  color = "default",
}: {
  name: string
  color?: VlanColor
}) {
  return (
    <Badge
      variant="outline"
      data-vlan-color={color}
      className="max-w-full"
      title={name}
    >
      <span className="truncate">{name}</span>
    </Badge>
  )
}
