"use client"

import { TablePagination } from "@/components/table-pagination"
import type { Filters, Sort } from "@/lib/list-controls"

type AuditLogPaginationProps = {
  filters: Filters
  sort: Sort
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
  filters,
  sort,
  limit,
  offset,
  total,
  pageSizes,
}: AuditLogPaginationProps) {
  const baseQuery = {
    ...filters,
    sort_by: sort.key,
    sort_dir: sort.direction,
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
