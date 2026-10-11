import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { TableLoadingPanel } from "@/components/loading-panels"
import { VlanManager } from "@/components/vlan-manager"
import { listDevices, listVlans } from "@/lib/api"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"

type SearchParams = Promise<Record<string, string | string[] | undefined>>

export const metadata = createPageMetadata("/networks/vlans")

export default async function VlansPage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams

  if (hasForcedGlimmer(params)) {
    return <TableLoadingPanel kind="vlans" />
  }

  const [vlans, devices] = await Promise.all([listVlans(), listDevices()])

  if (!vlans.ok) {
    if (vlans.status === 403) {
      return <ForbiddenState />
    }

    return <UnauthorizedState />
  }

  if (!devices.ok) {
    return devices.status === 403 ? <ForbiddenState /> : <UnauthorizedState />
  }

  return (
    <div className="px-4 lg:px-6">
      <VlanManager
        initialItems={vlans.data.items}
        devices={devices.data.items}
      />
    </div>
  )
}
