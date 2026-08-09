"use client"

import { TablePagination } from "@/components/table-pagination"

type AuditLogPaginationProps = {
  action: string
  resourceType: string
  resourceId: string
  actorUsername: string
  limit: number
  offset: number
  total: number
  pageSizes: readonly number[]
}

function buildQuery(values: Record<string, string | number | undefined>) {
  const params = new URLSearchParams()

  for (const [key, rawValue] of Object.entries(values)) {
    if (rawValue === undefined || rawValue === "") {
      continue
    }
    params.set(key, String(rawValue))
  }

  const query = params.toString()
  return query ? `/admin/audit-logs?${query}` : "/admin/audit-logs"
}

export function AuditLogPagination({
  action,
  resourceType,
  resourceId,
  actorUsername,
  limit,
  offset,
  total,
  pageSizes,
}: AuditLogPaginationProps) {
  const baseQuery = {
    action,
    resource_type: resourceType,
    resource_id: resourceId,
    actor_username: actorUsername,
  }

  return (
    <TablePagination
      pageSizeId="audit-page-size"
      limit={limit}
      offset={offset}
      total={total}
      pageSizes={pageSizes}
      buildHref={({ limit: nextLimit, offset: nextOffset }) =>
        buildQuery({ ...baseQuery, limit: nextLimit, offset: nextOffset })
      }
    />
  )
}
