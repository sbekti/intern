import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { DeviceManager } from "@/components/device-manager"
import { TableLoadingPanel } from "@/components/loading-panels"
import { listDevices, listVlans } from "@/lib/api"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"

type SearchParams = Promise<Record<string, string | string[] | undefined>>

export const metadata = createPageMetadata("/networks/devices")

export default async function DevicesPage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams

  if (hasForcedGlimmer(params)) {
    return <TableLoadingPanel titleWidth="w-20" />
  }

  const [devices, vlans] = await Promise.all([listDevices(), listVlans()])

  if (!devices.ok) {
    if (devices.status === 403) {
      return <ForbiddenState />
    }

    return <UnauthorizedState />
  }

  if (!vlans.ok) {
    if (vlans.status === 403) {
      return <ForbiddenState />
    }

    return <UnauthorizedState />
  }

  return (
    <div className="px-4 lg:px-6">
      <DeviceManager initialItems={devices.data.items} vlans={vlans.data.items} />
    </div>
  )
}
