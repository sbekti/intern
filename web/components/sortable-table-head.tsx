"use client"

import { ArrowDownIcon, ArrowUpIcon, ArrowUpDownIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { TableHead } from "@/components/ui/table"
import type { Sort } from "@/lib/list-controls"

export function SortableTableHead({
  children,
  sortKey,
  sort,
  onSort,
  disabled,
}: {
  children: string
  sortKey: string
  sort: Sort
  onSort: (key: string) => void
  disabled?: boolean
}) {
  const active = sort.key === sortKey
  const Icon = active
    ? sort.direction === "asc"
      ? ArrowUpIcon
      : ArrowDownIcon
    : ArrowUpDownIcon
  return (
    <TableHead
      aria-sort={
        active
          ? sort.direction === "asc"
            ? "ascending"
            : "descending"
          : "none"
      }
    >
      <Button
        variant="ghost"
        size="sm"
        className="-ml-2"
        disabled={disabled}
        aria-label={`Sort by ${children}`}
        onClick={() => onSort(sortKey)}
      >
        {children}
        <Icon data-icon="inline-end" />
      </Button>
    </TableHead>
  )
}
