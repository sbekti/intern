"use client"

import { useEffect, useEffectEvent, useState } from "react"
import { SearchIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty"
import { Field, FieldGroup } from "@/components/ui/field"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { FilterField, Filters } from "@/lib/list-controls"

export function NoMatchingRecords() {
  return (
    <Empty className="min-h-[16rem] border bg-muted/20">
      <EmptyHeader>
        <EmptyTitle>No matching records</EmptyTitle>
        <EmptyDescription>
          Change or clear the search and filters to see more records.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function ListToolbar({
  query,
  onQueryChange,
  searchLabel,
  fields,
  filters,
  onFiltersChange,
  onClear,
}: {
  query: string
  onQueryChange: (query: string) => void
  searchLabel: string
  fields: readonly FilterField[]
  filters: Filters
  onFiltersChange: (filters: Filters) => void
  onClear: () => void
}) {
  const [draft, setDraft] = useState({ source: query, value: query })
  if (draft.source !== query) {
    // A URL update can arrive after the user has already typed more text.
    setDraft({
      source: query,
      value: draft.value === draft.source ? query : draft.value,
    })
  }

  const commitSearch = useEffectEvent(onQueryChange)
  useEffect(() => {
    if (draft.value.trim() === query) return
    const timer = setTimeout(() => commitSearch(draft.value), 300)
    return () => clearTimeout(timer)
  }, [draft.value, query])

  useEffect(() => {
    function restoreSearch() {
      const query =
        new URLSearchParams(window.location.search).get("q")?.trim() ?? ""
      setDraft({ source: query, value: query })
    }
    window.addEventListener("popstate", restoreSearch)
    return () => window.removeEventListener("popstate", restoreSearch)
  }, [])

  return (
    <FieldGroup className="flex-row flex-wrap items-center gap-2">
      <Field className="min-w-48 flex-1 basis-full sm:basis-0">
        <InputGroup>
          <InputGroupAddon>
            <SearchIcon aria-hidden="true" />
          </InputGroupAddon>
          <InputGroupInput
            type="search"
            aria-label={searchLabel}
            placeholder={`${searchLabel}…`}
            value={draft.value}
            onChange={(event) =>
              setDraft({ source: query, value: event.target.value })
            }
          />
        </InputGroup>
      </Field>
      {fields.map((field) => {
        const options = [{ value: "", label: "All" }, ...field.options]
        const selected = filters[field.key]
        if (selected && !options.some(({ value }) => value === selected))
          options.push({ value: selected, label: selected })
        return (
          <Field key={field.key} className="w-auto">
            <Select
              items={options}
              value={filters[field.key] ?? ""}
              onValueChange={(value) =>
                onFiltersChange({ ...filters, [field.key]: value ?? "" })
              }
            >
              <SelectTrigger aria-label={`${field.label} filter`}>
                {field.label}:
                <SelectValue>
                  {(value) =>
                    options.find((option) => option.value === value)?.label ??
                    value
                  }
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {options.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
        )
      })}
      {draft.value.trim() || Object.values(filters).some(Boolean) ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-label="Clear search and filters"
          onClick={() => {
            setDraft({ source: query, value: "" })
            onClear()
          }}
        >
          Clear
        </Button>
      ) : null}
    </FieldGroup>
  )
}
