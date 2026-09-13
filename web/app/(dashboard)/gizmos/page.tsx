import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { GizmoManager } from "@/components/gizmo-manager"
import { TableLoadingPanel } from "@/components/loading-panels"
import { listDevices, listGizmos } from "@/lib/api"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"

type SearchParams = Promise<Record<string, string | string[] | undefined>>

export const metadata = createPageMetadata("/gizmos")

export default async function GizmosPage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams
  if (hasForcedGlimmer(params)) {
    return <TableLoadingPanel titleWidth="w-20" />
  }

  const [gizmos, devices] = await Promise.all([listGizmos(), listDevices()])
  if (!gizmos.ok) {
    return gizmos.status === 403 ? <ForbiddenState /> : <UnauthorizedState />
  }
  if (!devices.ok) {
    return devices.status === 403 ? <ForbiddenState /> : <UnauthorizedState />
  }

  return (
    <div className="px-4 lg:px-6">
      <GizmoManager
        initialItems={gizmos.data.items}
        devices={devices.data.items}
      />
    </div>
  )
}
