"use client"

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { Sort } from "@/lib/list-controls"

export function ListSort({
  sort,
  options,
  onSort,
  disabled,
}: {
  sort: Sort
  options: { key: string; label: string }[]
  onSort: (key: string, direction: "asc" | "desc") => void
  disabled?: boolean
}) {
  const items = options.flatMap(({ key, label }) => [
    { value: `${key}:asc`, label: `${label} · ascending` },
    { value: `${key}:desc`, label: `${label} · descending` },
  ])
  return (
    <Select
      value={`${sort.key}:${sort.direction}`}
      items={items}
      disabled={disabled}
      onValueChange={(value) => {
        if (!value) return
        const [key, direction] = value.split(":")
        if (direction === "asc" || direction === "desc") onSort(key, direction)
      }}
    >
      <SelectTrigger aria-label="Sort records" className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {items.map(({ value, label }) => (
            <SelectItem key={value} value={value}>
              {label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
