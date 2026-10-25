"use client"

import { TablePagination } from "@/components/table-pagination"
import { withQuery } from "@/lib/query-string"

type AuthSessionPaginationProps = {
  tab: "mine" | "all"
  limit: number
  offset: number
  total: number
  pageSizes: readonly number[]
}

export function AuthSessionPagination({
  tab,
  limit,
  offset,
  total,
  pageSizes,
}: AuthSessionPaginationProps) {
  return (
    <TablePagination
      pageSizeId={
        tab === "all" ? "admin-session-page-size" : "profile-session-page-size"
      }
      limit={limit}
      offset={offset}
      total={total}
      pageSizes={pageSizes}
      buildHref={({ limit: nextLimit, offset: nextOffset }) =>
        withQuery("/profile/security", {
          tab: tab === "all" ? "all" : undefined,
          limit: nextLimit,
          offset: nextOffset,
        })
      }
    />
  )
}
