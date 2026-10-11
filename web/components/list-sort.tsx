"use client"

import { useState } from "react"
import { ArrowDownIcon, ArrowUpIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer"
import { FieldGroup, FieldLegend, FieldSet } from "@/components/ui/field"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import type { Sort } from "@/lib/list-controls"

const textOrderLabels = { asc: "A–Z", desc: "Z–A" }

export function ListSort({
  sort,
  options,
  onSort,
  disabled,
}: {
  sort: Sort
  options: {
    key: string
    label: string
    orderLabels?: Record<Sort["direction"], string>
  }[]
  onSort: (key: string, direction: "asc" | "desc") => void
  disabled?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState(sort)
  const current = options.find(({ key }) => key === sort.key)
  const selected = options.find(({ key }) => key === draft.key)
  const currentOrderLabels = current?.orderLabels ?? textOrderLabels
  const orderLabels = selected?.orderLabels ?? textOrderLabels
  const Icon = sort.direction === "asc" ? ArrowUpIcon : ArrowDownIcon

  return (
    <Drawer
      autoFocus
      open={open}
      onOpenChange={(next) => {
        if (next) setDraft(sort)
        setOpen(next)
      }}
    >
      <DrawerTrigger asChild>
        <Button
          variant="outline"
          className="self-start"
          disabled={disabled}
          aria-label={`Sort by ${current?.label ?? sort.key}, ${currentOrderLabels[sort.direction]}`}
        >
          Sort: {current?.label ?? sort.key}
          <Icon data-icon="inline-end" aria-hidden="true" />
        </Button>
      </DrawerTrigger>
      <DrawerContent aria-describedby={undefined}>
        <DrawerHeader className="shrink-0">
          <DrawerTitle>Sort</DrawerTitle>
        </DrawerHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-4">
          <FieldGroup>
            <FieldSet>
              <FieldLegend>Sort by</FieldLegend>
              <ToggleGroup
                aria-label="Sort by"
                variant="outline"
                orientation="vertical"
                className="w-full"
                value={[draft.key]}
                disabled={disabled}
                onValueChange={(value) => {
                  if (value[0]) setDraft({ ...draft, key: value[0] })
                }}
              >
                {options.map(({ key, label }) => (
                  <ToggleGroupItem
                    key={key}
                    value={key}
                    className="justify-start"
                  >
                    {label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </FieldSet>
            <FieldSet>
              <FieldLegend>Order</FieldLegend>
              <ToggleGroup
                aria-label="Order"
                variant="outline"
                className="w-full"
                value={[draft.direction]}
                disabled={disabled}
                onValueChange={(value) => {
                  const direction = value[0]
                  if (direction === "asc" || direction === "desc")
                    setDraft({ ...draft, direction })
                }}
              >
                <ToggleGroupItem value="asc" className="flex-1">
                  {orderLabels.asc}
                </ToggleGroupItem>
                <ToggleGroupItem value="desc" className="flex-1">
                  {orderLabels.desc}
                </ToggleGroupItem>
              </ToggleGroup>
            </FieldSet>
          </FieldGroup>
        </div>
        <DrawerFooter className="shrink-0">
          <Button
            disabled={disabled}
            onClick={() => {
              setOpen(false)
              if (draft.key !== sort.key || draft.direction !== sort.direction)
                onSort(draft.key, draft.direction)
            }}
          >
            Apply
          </Button>
          <DrawerClose asChild>
            <Button variant="outline">Cancel</Button>
          </DrawerClose>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}
