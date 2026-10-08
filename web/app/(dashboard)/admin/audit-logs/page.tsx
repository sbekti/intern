import { AuditLogTable } from "@/components/audit-log-table"
import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { AuditLogsLoadingPanel } from "@/components/loading-panels"
import { listAdminAuditLogs } from "@/lib/api"
import { auditSortKeys, defaultAuditSort } from "@/lib/audit-log-list"
import { readSort } from "@/lib/list-controls"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"

type SearchParams = Promise<Record<string, string | string[] | undefined>>

const defaultPageSize = 25
const allowedPageSizes = [25, 50, 100, 200] as const

export const metadata = createPageMetadata("/admin/audit-logs")

function readParam(
  params: Record<string, string | string[] | undefined>,
  key: string
) {
  const value = params[key]
  if (Array.isArray(value)) {
    return value[0] ?? ""
  }
  return value ?? ""
}

function parseOffset(value: string) {
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0
  }
  return parsed
}

function parsePageSize(value: string) {
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed)) {
    return defaultPageSize
  }
  if (allowedPageSizes.includes(parsed as (typeof allowedPageSizes)[number])) {
    return parsed
  }
  return defaultPageSize
}

export default async function AuditLogsPage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams
  const action = readParam(params, "action")
  const resourceType = readParam(params, "resource_type")
  const resourceId = readParam(params, "resource_id")
  const actorUsername = readParam(params, "actor_username")
  const sort = readSort(
    { get: (key) => readParam(params, key) },
    auditSortKeys,
    defaultAuditSort
  )
  const limit = parsePageSize(readParam(params, "limit"))
  const offset = parseOffset(readParam(params, "offset"))

  if (hasForcedGlimmer(params)) {
    return <AuditLogsLoadingPanel />
  }

  const auditLogs = await listAdminAuditLogs({
    action,
    resource_type: resourceType,
    resource_id: resourceId,
    actor_username: actorUsername,
    sort_by: sort.key,
    sort_dir: sort.direction,
    limit,
    offset,
  })

  if (!auditLogs.ok) {
    if (auditLogs.status === 401) {
      return <UnauthorizedState />
    }

    return <ForbiddenState />
  }

  return (
    <div className="px-4 lg:px-6">
      <AuditLogTable page={auditLogs.data} pageSizes={allowedPageSizes} />
    </div>
  )
}
