"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { SearchIcon, XIcon } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  useComboboxAnchor,
  useComboboxFilter,
} from "@/components/ui/combobox"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  InputGroup,
  InputGroupInput,
  InputGroupAddon,
  InputGroupButton,
} from "@/components/ui/input-group"
import type { FilterField, FilterOption, Filters } from "@/lib/list-controls"

type Picker = { fieldKey: string | null; query: string }
const inputLayout =
  "h-auto min-h-9 min-w-0 flex-1 flex-wrap gap-y-1 [&_[data-slot=input-group-control]]:h-8 [&_[data-slot=input-group-control]]:min-w-24"

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
}: {
  fields: readonly FilterField[]
  filters: Filters
  onChange: (filters: Filters) => void
}) {
  const [picker, setPickerState] = useState<Picker | null>(null)
  // Selection and blur can run before React renders the previous event's update.
  const draft = useRef<Picker | null>(null)
  const focusOnMount = useRef(false)
  const setInput = useCallback((element: HTMLInputElement | null) => {
    if (element && focusOnMount.current) {
      focusOnMount.current = false
      element.focus()
    }
  }, [])
  const anchor = useComboboxAnchor()
  const { contains } = useComboboxFilter({ sensitivity: "base" })
  const field = fields.find((f) => f.key === picker?.fieldKey)
  const activeFields = fields.filter((f) => filters[f.key])
  const textField = field && !field.options
  const choices: readonly FilterOption[] =
    field?.options ??
    fields
      .filter((f) => !filters[f.key])
      .map((f) => ({ value: f.key, label: f.label }))

  function setPicker(next: Picker | null) {
    draft.current = next
    setPickerState(next)
  }

  useEffect(() => {
    const cancel = () => {
      draft.current = null
      setPickerState(null)
    }
    window.addEventListener("popstate", cancel)
    return () => window.removeEventListener("popstate", cancel)
  }, [])

  function finish(focus = false): Filters {
    const current = draft.current
    const field = fields.find((f) => f.key === current?.fieldKey)
    const value = current?.query.trim()
    focusOnMount.current = focus
    setPicker(null)
    return field && !field.options && value
      ? { ...filters, [field.key]: value }
      : filters
  }

  function edit(field: FilterField) {
    const next = finish()
    onChange(next)
    focusOnMount.current = true
    setPicker({
      fieldKey: field.key,
      query:
        field.options?.find((o) => o.value === next[field.key])?.label ??
        next[field.key] ??
        "",
    })
  }

  const tokens = (
    <InputGroupAddon className="max-w-full flex-wrap justify-start py-0">
      <SearchIcon aria-hidden="true" />
      {activeFields.map((f) => {
        const label = `${f.label}: ${f.options?.find((o) => o.value === filters[f.key])?.label ?? filters[f.key]}`
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
              aria-label={`Edit ${label} filter`}
              title={label}
              onPointerDown={(event) => event.preventDefault()}
              onClick={() => edit(f)}
            >
              <span className="truncate">{label}</span>
            </InputGroupButton>
            <InputGroupButton
              size="icon-xs"
              variant="ghost"
              aria-label={`Remove ${f.label} filter`}
              onPointerDown={(event) => event.preventDefault()}
              onClick={() => {
                setPicker(null)
                const next = { ...filters }
                delete next[f.key]
                onChange(next)
              }}
            >
              <XIcon data-icon="inline-end" />
            </InputGroupButton>
          </Badge>
        )
      })}
      {field ? <span>{field.label}:</span> : null}
    </InputGroupAddon>
  )

  return (
    <div
      ref={anchor}
      className="flex min-w-0 items-start gap-2"
      data-slot="token-filter-bar"
    >
      {textField ? (
        <InputGroup className={inputLayout}>
          <InputGroupInput
            autoFocus
            ref={setInput}
            aria-label={`${field.label} filter value`}
            aria-description={`${field.exact ? "Matches the exact value." : "Matches text anywhere, ignoring case."} Press Enter or leave the field to apply.`}
            placeholder={
              field.exact ? "Type an exact value…" : "Type text to match…"
            }
            value={picker?.query ?? ""}
            onChange={(event) =>
              setPicker({ fieldKey: field.key, query: event.target.value })
            }
            onBlur={() => onChange(finish())}
            onKeyDown={(event) => {
              if (event.key === "Escape") {
                event.preventDefault()
                focusOnMount.current = true
                setPicker(null)
              } else if (
                event.key === "Enter" &&
                !event.nativeEvent.isComposing
              ) {
                event.preventDefault()
                onChange(finish(true))
              }
            }}
          />
          {tokens}
        </InputGroup>
      ) : (
        <Combobox<FilterOption>
          items={choices}
          filteredItems={choices.filter((choice) =>
            contains(choice.label, picker?.query.trim() ?? "")
          )}
          value={null}
          itemToStringLabel={(item) => item.label}
          inputValue={picker?.query ?? ""}
          onInputValueChange={(value, details) => {
            if (
              details.reason === "input-change" ||
              details.reason === "input-clear"
            )
              setPicker({
                fieldKey: draft.current?.fieldKey ?? null,
                query: value,
              })
          }}
          open={picker !== null}
          autoHighlight
          onOpenChange={(next, details) => {
            if (!next && details.reason === "item-press") {
              details.cancel()
              return
            }
            if (next) {
              if (!draft.current) setPicker({ fieldKey: null, query: "" })
            } else setPicker(null)
          }}
          onValueChange={(choice) => {
            if (!choice) return
            if (!field) {
              setPicker({ fieldKey: choice.value, query: "" })
            } else {
              setPicker(null)
              onChange({ ...filters, [field.key]: choice.value })
            }
          }}
        >
          <ComboboxInput
            ref={setInput}
            className={inputLayout}
            showTrigger={false}
            aria-label={field ? `${field.label} filter value` : "Add filter"}
            placeholder={field ? "Choose a value…" : "Add filter…"}
          >
            {tokens}
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
                  {choice.label}
                </ComboboxItem>
              )}
            </ComboboxList>
          </ComboboxContent>
        </Combobox>
      )}
      {activeFields.length || (textField && picker?.query.trim()) ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onPointerDown={(event) => event.preventDefault()}
          onClick={() => {
            setPicker(null)
            onChange({})
          }}
        >
          Clear
        </Button>
      ) : null}
    </div>
  )
}
