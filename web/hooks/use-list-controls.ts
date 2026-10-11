"use client"

import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { useTransition } from "react"
import {
  readFilters,
  readSort,
  updateListQuery,
  type Filters,
  type Sort,
} from "@/lib/list-controls"

export function useListControls(
  filterKeys: readonly string[],
  sortKeys: readonly string[],
  fallback: Sort,
  server = false
) {
  const params = useSearchParams()
  const pathname = usePathname()
  const router = useRouter()
  const [pending, startTransition] = useTransition()
  const filters = readFilters(params, filterKeys)
  const sort = readSort(params, sortKeys, fallback)

  function update(changes: Record<string, string | undefined>) {
    const query = updateListQuery(window.location.search, changes, server)
    const href = pathname + (query ? `?${query}` : "") + window.location.hash
    if (href === pathname + window.location.search + window.location.hash)
      return
    if (server) startTransition(() => router.push(href, { scroll: false }))
    else window.history.pushState(null, "", href)
  }

  function setFilters(next: Filters) {
    if (filterKeys.every((key) => next[key] === filters[key])) return
    update(Object.fromEntries(filterKeys.map((key) => [key, next[key]])))
  }

  function setSort(key: string, direction?: "asc" | "desc") {
    update({
      sort_by: key,
      sort_dir:
        direction ??
        (sort.key === key && sort.direction === "asc" ? "desc" : "asc"),
    })
  }

  return { filters, sort, pending, setFilters, setSort }
}
