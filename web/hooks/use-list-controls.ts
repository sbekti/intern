"use client"

import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { useOptimistic, useTransition } from "react"
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
  const [optimisticQuery, setOptimisticQuery] = useOptimistic(params.toString())
  const listParams = server ? new URLSearchParams(optimisticQuery) : params
  const filters = readFilters(listParams, filterKeys)
  const query = listParams.get("q")?.trim() ?? ""
  const sort = readSort(listParams, sortKeys, fallback)

  function update(
    changes: Record<string, string | undefined>,
    replace = false
  ) {
    const current = server ? optimisticQuery : window.location.search
    const query = updateListQuery(current, changes, server)
    const href = pathname + (query ? `?${query}` : "") + window.location.hash
    if (query === new URLSearchParams(current).toString()) return
    if (server)
      startTransition(() => {
        setOptimisticQuery(query)
        if (replace) router.replace(href, { scroll: false })
        else router.push(href, { scroll: false })
      })
    else window.history[replace ? "replaceState" : "pushState"](null, "", href)
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

  function setQuery(value: string) {
    update({ q: value.trim() }, true)
  }

  function clear() {
    update({
      ...Object.fromEntries(filterKeys.map((key) => [key, undefined])),
      q: undefined,
    })
  }

  return { filters, query, sort, pending, setFilters, setQuery, setSort, clear }
}
