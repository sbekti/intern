import { AuditLogTable } from "@/components/audit-log-table"
import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { AuditLogsLoadingPanel } from "@/components/loading-panels"
import { listAdminAuditLogs } from "@/lib/api"
import {
  auditFilterKeys,
  auditSortKeys,
  defaultAuditSort,
} from "@/lib/audit-log-list"
import { readFilters, readSort } from "@/lib/list-controls"
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
  return allowedPageSizes.find((size) => size === parsed) ?? defaultPageSize
}

export default async function AuditLogsPage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams
  const query = { get: (key: string) => readParam(params, key) }
  const filters = readFilters(query, auditFilterKeys)
  const sort = readSort(query, auditSortKeys, defaultAuditSort)
  const limit = parsePageSize(query.get("limit"))
  const offset = parseOffset(query.get("offset"))

  if (hasForcedGlimmer(params)) {
    return <AuditLogsLoadingPanel />
  }

  const auditLogs = await listAdminAuditLogs({
    ...filters,
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
