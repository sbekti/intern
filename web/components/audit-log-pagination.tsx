"use client"

import { TablePagination } from "@/components/table-pagination"
import { withQuery } from "@/lib/query-string"
import type { Filters, Sort } from "@/lib/list-controls"

type AuditLogPaginationProps = {
  filters: Filters
  sort: Sort
  limit: number
  offset: number
  total: number
  pageSizes: readonly number[]
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
        withQuery("/admin/audit-logs", {
          ...baseQuery,
          limit: nextLimit,
          offset: nextOffset,
        })
      }
    />
  )
}
