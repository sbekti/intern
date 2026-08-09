"use client"

import { TablePagination } from "@/components/table-pagination"

type AuthSessionPaginationProps = {
  tab: "mine" | "all"
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
  return query ? `/profile/security?${query}` : "/profile/security"
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
      pageSizeId={tab === "all" ? "admin-session-page-size" : "profile-session-page-size"}
      limit={limit}
      offset={offset}
      total={total}
      pageSizes={pageSizes}
      buildHref={({ limit: nextLimit, offset: nextOffset }) =>
        buildQuery({
          tab: tab === "all" ? "all" : undefined,
          limit: nextLimit,
          offset: nextOffset,
        })
      }
    />
  )
}
