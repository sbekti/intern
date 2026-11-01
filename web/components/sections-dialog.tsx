"use client"

import { useRef, useState } from "react"
import { PointerSensor, PointerActivationConstraints } from "@dnd-kit/dom"
import { DragDropProvider } from "@dnd-kit/react"
import { isSortable, useSortable } from "@dnd-kit/react/sortable"
import { GripVerticalIcon, SlidersHorizontalIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Field,
  FieldGroup,
  FieldError,
  FieldLabel,
  FieldSet,
  FieldLegend,
} from "@/components/ui/field"
import {
  defaultDashboardLayout,
  moveSection,
  orderedSections,
  type DashboardLayout,
} from "@/lib/dashboard-layout"
import type { ClientMetricGroup } from "@/lib/metrics-config"
import { mutate } from "@/lib/mutations"
import { buildBffPath } from "@/lib/bff"

const sectionSensors = [
  PointerSensor.configure({
    activationConstraints: (event) =>
      event.pointerType === "touch"
        ? [
            new PointerActivationConstraints.Delay({
              value: 250,
              tolerance: 5,
            }),
          ]
        : [new PointerActivationConstraints.Distance({ value: 5 })],
  }),
]

function SectionRow({
  group,
  index,
  count,
  checked,
  busy,
  onToggle,
  onMove,
  draggedRef,
}: {
  group: ClientMetricGroup
  index: number
  count: number
  checked: boolean
  busy: boolean
  onToggle: () => void
  onMove: (to: number) => void
  draggedRef: { current: boolean }
}) {
  const { ref, handleRef } = useSortable({
    id: group.id,
    index,
    disabled: busy,
  })
  const [menuOpen, setMenuOpen] = useState(false)
  return (
    <Field ref={ref} orientation="horizontal">
      <DropdownMenu
        open={menuOpen}
        onOpenChange={(open, details) => {
          // Open on click, allowing a press-and-drag to remain a drag.
          if (details.event.type !== "mousedown") setMenuOpen(open)
        }}
      >
        <DropdownMenuTrigger
          ref={handleRef}
          render={<Button type="button" variant="ghost" size="icon" />}
          disabled={busy}
          aria-label={`Reorder ${group.title}`}
          className="cursor-grab touch-none"
          onPointerDown={() => {
            draggedRef.current = false
          }}
          onClick={(event) => {
            if (event.detail > 0 && !draggedRef.current) setMenuOpen(true)
          }}
        >
          <GripVerticalIcon />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuGroup>
            <DropdownMenuItem
              disabled={index === 0}
              onClick={() => onMove(index - 1)}
            >
              Move up
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={index === count - 1}
              onClick={() => onMove(index + 1)}
            >
              Move down
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
      <Checkbox
        id={`section-${group.id}`}
        checked={checked}
        disabled={busy}
        onCheckedChange={onToggle}
      />
      <FieldLabel htmlFor={`section-${group.id}`}>{group.title}</FieldLabel>
    </Field>
  )
}

export function SectionsDialog({
  groups,
  layout,
  onSave,
}: {
  groups: readonly ClientMetricGroup[]
  layout: DashboardLayout
  onSave: (layout: DashboardLayout) => void
}) {
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState(layout)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [announcement, setAnnouncement] = useState("")
  const draggedRef = useRef(false)
  const ordered = orderedSections(groups, draft)
  function move(from: number, to: number) {
    const next = moveSection(ordered, from, to)
    setDraft((previous) => ({
      ...previous,
      section_order: next.map((group) => group.id),
    }))
    setAnnouncement(
      `${ordered[from].title} moved from position ${from + 1} to ${to + 1}.`
    )
  }
  function toggle(id: string) {
    setDraft((previous) => ({
      ...previous,
      hidden_sections: previous.hidden_sections.includes(id)
        ? previous.hidden_sections.filter((hidden) => hidden !== id)
        : [...previous.hidden_sections, id],
    }))
  }
  async function save() {
    setBusy(true)
    setError(null)
    try {
      await mutate(buildBffPath("/preferences"), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ dashboard: draft }),
      })
      onSave(draft)
      setOpen(false)
    } catch (error) {
      setError(
        error instanceof Error
          ? error.message
          : "Could not save sections. Try again."
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (busy) return
        if (next) {
          setDraft(layout)
          setError(null)
          setAnnouncement("")
        }
        setOpen(next)
      }}
    >
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <SlidersHorizontalIcon data-icon="inline-start" />
        Sections
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Sections</DialogTitle>
          <DialogDescription>
            Show and arrange sections for everyone.
          </DialogDescription>
        </DialogHeader>
        <DragDropProvider
          sensors={sectionSensors}
          onDragStart={() => {
            draggedRef.current = true
          }}
          onDragEnd={(event) => {
            if (event.canceled) return
            const { source } = event.operation
            if (isSortable(source) && source.initialIndex !== source.index)
              move(source.initialIndex, source.index)
          }}
        >
          <FieldSet className="max-h-[60dvh] overflow-y-auto">
            <FieldLegend className="sr-only">Dashboard sections</FieldLegend>
            <FieldGroup className="gap-2">
              {ordered.map((group, index) => (
                <SectionRow
                  key={group.id}
                  group={group}
                  index={index}
                  count={ordered.length}
                  checked={!draft.hidden_sections.includes(group.id)}
                  busy={busy}
                  draggedRef={draggedRef}
                  onMove={(to) => move(index, to)}
                  onToggle={() => toggle(group.id)}
                />
              ))}
            </FieldGroup>
          </FieldSet>
        </DragDropProvider>
        <p className="sr-only" aria-live="polite">
          {announcement}
        </p>
        {error && <FieldError role="alert">{error}</FieldError>}
        <DialogFooter>
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => setDraft(defaultDashboardLayout)}
          >
            Reset
          </Button>
          <Button
            variant="outline"
            disabled={busy}
            onClick={() => setOpen(false)}
          >
            Cancel
          </Button>
          <Button disabled={busy} onClick={save}>
            {busy ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
