export type FilterOption = { value: string; label: string }
export type FilterField = {
  key: string
  label: string
  options: readonly FilterOption[]
  allowCustom?: boolean
}
export type Filters = Record<string, string>
export type Sort<Key extends string = string> = {
  key: Key
  direction: "asc" | "desc"
}
export type SortValue = string | number | null

export function filterOptions(values: readonly string[]): FilterOption[] {
  return [...new Set(values)]
    .sort(compareText)
    .map((value) => ({ value, label: value }))
}

export function readFilters(
  params: Pick<URLSearchParams, "get">,
  keys: readonly string[]
): Filters {
  return Object.fromEntries(
    keys.flatMap((key) => {
      const value = params.get(key)?.trim()
      return value ? [[key, value]] : []
    })
  )
}

export function readSort<Key extends string>(
  params: Pick<URLSearchParams, "get">,
  keys: readonly Key[],
  fallback: Sort<Key>
): Sort<Key> {
  const key = keys.find(
    (key) => key === (params.get("sort_by") || fallback.key)
  )
  const direction = params.get("sort_dir") || fallback.direction
  return key !== undefined && (direction === "asc" || direction === "desc")
    ? { key, direction }
    : fallback
}

export function updateListQuery(
  current: string,
  changes: Record<string, string | undefined>,
  resetOffset = false
) {
  const params = new URLSearchParams(current)
  for (const [key, value] of Object.entries(changes)) {
    if (value) params.set(key, value)
    else params.delete(key)
  }
  if (resetOffset) params.delete("offset")
  return params.toString()
}

function compareText(a: string, b: string) {
  return a.localeCompare(b, undefined, { sensitivity: "base" })
}

export function filterAndSort<T>(
  items: readonly T[],
  filters: Filters,
  fields: Record<string, (item: T) => string>,
  sort: Sort,
  sorts: Record<string, (item: T) => SortValue>,
  id: (item: T) => string | number
) {
  return items
    .filter((item) =>
      Object.entries(filters).every(
        ([key, value]) => fields[key](item) === value
      )
    )
    .sort((a, b) => {
      const left = sorts[sort.key](a),
        right = sorts[sort.key](b)
      // Unconfigured destinations stay last in either direction.
      if (left === null && right !== null) return 1
      if (right === null && left !== null) return -1
      const order =
        typeof left === "number" && typeof right === "number"
          ? left - right
          : compareText(String(left ?? ""), String(right ?? ""))
      const aId = id(a),
        bId = id(b)
      const tie =
        typeof aId === "number" && typeof bId === "number"
          ? aId - bId
          : compareText(String(aId), String(bId))
      return order * (sort.direction === "asc" ? 1 : -1) || tie
    })
}
