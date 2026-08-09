import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { TableLoadingPanel } from "@/components/loading-panels"
import { VlanManager } from "@/components/vlan-manager"
import { listVlans } from "@/lib/api"
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
    return <TableLoadingPanel titleWidth="w-16" />
  }

  const vlans = await listVlans()

  if (!vlans.ok) {
    if (vlans.status === 403) {
      return <ForbiddenState />
    }

    return <UnauthorizedState />
  }

  return (
    <div className="px-4 lg:px-6">
      <VlanManager initialItems={vlans.data.items} />
    </div>
  )
}
