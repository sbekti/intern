"use client"

import { useRef, useState } from "react"
import { SearchIcon, XIcon } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { InputGroupAddon, InputGroupButton } from "@/components/ui/input-group"
import {
  Combobox,
  ComboboxInput,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxItem,
  ComboboxList,
  useComboboxAnchor,
  useComboboxFilter,
} from "@/components/ui/combobox"
import type { FilterField, FilterOption, Filters } from "@/lib/list-controls"

export function NoMatchingRecords() {
  return (
    <Empty className="min-h-[16rem] border bg-muted/20">
      <EmptyHeader>
        <EmptyTitle>No matching records</EmptyTitle>
        <EmptyDescription>
          Change or clear the filters to see more records.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function TokenFilterBar({
  fields,
  filters,
  onChange,
  disabled = false,
}: {
  fields: readonly FilterField[]
  filters: Filters
  onChange: (filters: Filters) => void
  disabled?: boolean
}) {
  const [fieldKey, setFieldKey] = useState<string | null>(null)
  const [query, setQuery] = useState("")
  const [open, setOpen] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const anchor = useComboboxAnchor()
  const { contains } = useComboboxFilter({ sensitivity: "base" })
  const search = query.trim()
  const field = fields.find((f) => f.key === fieldKey)
  const activeFields = fields.filter((f) => filters[f.key])
  const choices: FilterOption[] = field
    ? [...field.options]
    : fields
        .filter((f) => !filters[f.key])
        .map((f) => ({ value: f.key, label: f.label }))
  if (
    field?.allowCustom &&
    search &&
    !field.options.some((o) => o.value === search)
  ) {
    choices.push({
      value: search,
      label: `Use “${search}”`,
    })
  }

  function closePicker() {
    setFieldKey(null)
    setQuery("")
    setOpen(false)
  }
  function edit(f: FilterField) {
    setFieldKey(f.key)
    setQuery(
      f.options.find((o) => o.value === filters[f.key])?.label ?? filters[f.key]
    )
    setOpen(true)
    input.current?.focus()
  }
  function remove(key: string) {
    const next = { ...filters }
    delete next[key]
    closePicker()
    onChange(next)
  }

  return (
    <div
      ref={anchor}
      className="flex min-w-0 items-start gap-2"
      aria-busy={disabled}
      data-slot="token-filter-bar"
    >
      <Combobox<FilterOption>
        items={choices}
        filteredItems={choices.filter((choice) =>
          contains(choice.label, search)
        )}
        value={null}
        disabled={disabled}
        itemToStringLabel={(item) => item.label}
        inputValue={query}
        onInputValueChange={(value, details) => {
          if (
            details.reason === "input-change" ||
            details.reason === "input-clear"
          )
            setQuery(value)
        }}
        open={open}
        autoHighlight
        onOpenChange={(next, details) => {
          if (!next && details.reason === "item-press" && !field) {
            details.cancel()
            return
          }
          if (next) setOpen(true)
          else closePicker()
        }}
        onValueChange={(choice) => {
          if (!choice) return
          if (!field) {
            setFieldKey(choice.value)
            setQuery("")
            setOpen(true)
            input.current?.focus()
          } else {
            onChange({ ...filters, [field.key]: choice.value })
            closePicker()
          }
        }}
      >
        <ComboboxInput
          ref={input}
          className="h-auto min-h-9 min-w-0 flex-1 flex-wrap gap-y-1 [&_[data-slot=input-group-control]]:h-8 [&_[data-slot=input-group-control]]:min-w-24"
          showTrigger={false}
          aria-label={field ? `${field.label} filter value` : "Add filter"}
          placeholder={field ? "Choose a value…" : "Add filter…"}
        >
          <InputGroupAddon className="max-w-full flex-wrap justify-start py-0">
            <SearchIcon aria-hidden="true" />
            {activeFields.map((f) => {
              const label = `${f.label}: ${f.options.find((o) => o.value === filters[f.key])?.label ?? filters[f.key]}`
              return (
                <Badge
                  key={f.key}
                  variant="secondary"
                  className="h-7 max-w-full min-w-0"
                >
                  <InputGroupButton
                    size="xs"
                    variant="ghost"
                    className="min-w-0 shrink"
                    disabled={disabled}
                    aria-label={`Edit ${label} filter`}
                    title={label}
                    onClick={() => edit(f)}
                  >
                    <span className="truncate">{label}</span>
                  </InputGroupButton>
                  <InputGroupButton
                    variant="ghost"
                    size="icon-xs"
                    disabled={disabled}
                    aria-label={`Remove ${f.label} filter`}
                    onClick={() => remove(f.key)}
                  >
                    <XIcon data-icon="inline-end" />
                  </InputGroupButton>
                </Badge>
              )
            })}
            {field ? <span>{field.label}:</span> : null}
          </InputGroupAddon>
        </ComboboxInput>
        <ComboboxContent anchor={anchor}>
          <ComboboxEmpty>
            {field
              ? "No matching values."
              : activeFields.length === fields.length
                ? "All filters added. Click a token to edit it."
                : "No matching fields."}
          </ComboboxEmpty>
          <ComboboxList>
            {(choice: FilterOption) => (
              <ComboboxItem key={choice.value} value={choice}>
                <span className="truncate" title={choice.label}>
                  {choice.label}
                </span>
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
      {activeFields.length ? (
        <Button
          variant="ghost"
          size="sm"
          disabled={disabled}
          onClick={() => {
            closePicker()
            onChange({})
          }}
        >
          Clear
        </Button>
      ) : null}
    </div>
  )
}
