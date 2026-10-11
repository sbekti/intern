"use client"

import { ListTable, ListMobile } from "@/components/responsive-list"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { startTransition, useState } from "react"
import { toast } from "@/components/ui/toast"
import { NetworkIcon, PlusIcon } from "lucide-react"

import { GuardedDialog } from "@/components/guarded-dialog"
import { RecordActions } from "@/components/record-actions"
import { ListSort } from "@/components/list-sort"
import {
  Item,
  ItemContent,
  ItemTitle,
  ItemDescription,
  ItemFooter,
  ItemGroup,
} from "@/components/ui/item"
import { mutate } from "@/lib/mutations"
import type { NetworkDevice, Vlan } from "@/lib/api"
import { useListControls } from "@/hooks/use-list-controls"
import { filterAndSort } from "@/lib/list-controls"
import {
  vlanFilterFields,
  vlanFilterValues,
  vlanSortValues,
} from "@/lib/network-list"
import {
  NoMatchingRecords,
  TokenFilterBar,
} from "@/components/token-filter-bar"
import { SortableTableHead } from "@/components/sortable-table-head"
import { buildBffPath } from "@/lib/bff"
import { vlanColors, type VlanColor } from "@/lib/vlan-colors"
import { VlanBadge } from "@/components/vlan-badge"
import { responsiveCompactButtonClass } from "@/components/compact-button-label"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldTitle,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

type VlanFormState = {
  name: string
  vlan_id: string
  description: string
  color: VlanColor
}

const defaultFormState: VlanFormState = {
  name: "",
  vlan_id: "",
  description: "",
  color: "default",
}

function mapVlanToForm(vlan: Vlan): VlanFormState {
  return {
    name: vlan.name,
    vlan_id: String(vlan.vlan_id),
    description: vlan.description,
    color: vlan.color ?? "default",
  }
}

export function VlanManager({
  initialItems,
  devices,
}: {
  devices: NetworkDevice[]
  initialItems: Vlan[]
}) {
  const router = useRouter()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Vlan | null>(null)
  const [deleting, setDeleting] = useState<Vlan | null>(null)
  const [form, setForm] = useState<VlanFormState>(defaultFormState)
  const [initialForm, setInitialForm] = useState(defaultFormState)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [deletingBusy, setDeletingBusy] = useState(false)

  const controls = useListControls(
    Object.keys(vlanFilterValues),
    Object.keys(vlanSortValues),
    { key: "vlan_id", direction: "asc" }
  )
  const sortedItems = filterAndSort(
    initialItems,
    controls.filters,
    vlanFilterValues,
    controls.sort,
    vlanSortValues,
    (item) => item.vlan_id
  )
  const filterFields = vlanFilterFields(initialItems)

  function openCreate() {
    setEditing(null)
    setForm(defaultFormState)
    setInitialForm(defaultFormState)
    setSubmitError(null)
    setDialogOpen(true)
  }

  function openEdit(vlan: Vlan) {
    setEditing(vlan)
    setForm(mapVlanToForm(vlan))
    setInitialForm(mapVlanToForm(vlan))
    setSubmitError(null)
    setDialogOpen(true)
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setSubmitError(null)

    const payload = {
      name: form.name.trim(),
      vlan_id: Number(form.vlan_id),
      description: form.description.trim(),
      color: form.color,
    }

    try {
      await mutate(
        editing
          ? buildBffPath(`/networks/vlans/${editing.vlan_id}`)
          : buildBffPath("/networks/vlans"),
        {
          method: editing ? "PATCH" : "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify(payload),
        }
      )
      toast.add({
        type: "success",
        title: editing ? "VLAN updated" : "VLAN created",
      })
      setDialogOpen(false)
      startTransition(() => router.refresh())
    } catch (error) {
      setSubmitError(
        error instanceof Error
          ? error.message
          : "Couldn't save the VLAN. Try again."
      )
    } finally {
      setSubmitting(false)
    }
  }

  async function handleDelete() {
    if (!deleting) return
    setDeletingBusy(true)
    try {
      await mutate(buildBffPath(`/networks/vlans/${deleting.vlan_id}`), {
        method: "DELETE",
      })
      toast.add({ type: "success", title: "VLAN deleted" })
      setDeleting(null)
      startTransition(() => router.refresh())
    } catch (error) {
      toast.add({
        type: "error",
        title:
          error instanceof Error ? error.message : "Couldn't delete the VLAN.",
      })
    } finally {
      setDeletingBusy(false)
    }
  }

  const deviceCounts = new Map<number, number>()
  for (const device of devices)
    deviceCounts.set(
      device.vlan.vlan_id,
      (deviceCounts.get(device.vlan.vlan_id) ?? 0) + 1
    )
  function deviceCount(vlan: Vlan) {
    const count = deviceCounts.get(vlan.vlan_id) ?? 0
    return (
      <Link
        className="underline underline-offset-4"
        href={`/networks/devices?vlan_id=${vlan.vlan_id}`}
        aria-label={`View ${count} devices in ${vlan.name}`}
      >
        {count} {count === 1 ? "device" : "devices"}
      </Link>
    )
  }
  function vlanActions(vlan: Vlan) {
    return (
      <RecordActions
        name={vlan.name}
        onEdit={() => openEdit(vlan)}
        onDelete={() => setDeleting(vlan)}
        activityHref={`/admin/audit-logs?resource_type=vlan&resource_id=${vlan.vlan_id}`}
      />
    )
  }

  return (
    <>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>VLANs</CardTitle>
          <CardDescription>
            {sortedItems.length} of {initialItems.length} VLANs
          </CardDescription>
          <CardAction className="ml-3 sm:ml-0">
            <Button
              size="icon-sm"
              className={responsiveCompactButtonClass}
              onClick={openCreate}
              aria-label="Add VLAN"
            >
              <PlusIcon data-icon="inline-start" />
              <span className="hidden sm:inline">Add VLAN</span>
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <TokenFilterBar
            fields={filterFields}
            filters={controls.filters}
            onChange={controls.setFilters}
          />
          {initialItems.length > 0 && sortedItems.length === 0 ? (
            <NoMatchingRecords />
          ) : initialItems.length === 0 ? (
            <Empty className="min-h-[16rem] border bg-muted/20">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <NetworkIcon />
                </EmptyMedia>
                <EmptyTitle>No VLANs</EmptyTitle>
                <EmptyDescription>
                  Create the first VLAN definition to start assigning devices.
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={openCreate}>
                  <PlusIcon data-icon="inline-start" />
                  Add VLAN
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            <>
              <ListTable>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <SortableTableHead
                        sortKey="vlan_id"
                        sort={controls.sort}
                        onSort={controls.setSort}
                      >
                        VLAN ID
                      </SortableTableHead>
                      <SortableTableHead
                        sortKey="name"
                        sort={controls.sort}
                        onSort={controls.setSort}
                      >
                        Name
                      </SortableTableHead>
                      <SortableTableHead
                        sortKey="description"
                        sort={controls.sort}
                        onSort={controls.setSort}
                      >
                        Description
                      </SortableTableHead>
                      <TableHead>Devices</TableHead>
                      <TableHead className="text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sortedItems.map((vlan) => (
                      <TableRow key={vlan.vlan_id}>
                        <TableCell>{vlan.vlan_id}</TableCell>
                        <TableCell>
                          <VlanBadge name={vlan.name} color={vlan.color} />
                        </TableCell>
                        <TableCell className="max-w-[24rem] whitespace-normal text-muted-foreground">
                          {vlan.description || "-"}
                        </TableCell>
                        <TableCell>{deviceCount(vlan)}</TableCell>
                        <TableCell className="text-right">
                          {vlanActions(vlan)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </ListTable>
              <ListMobile>
                <ListSort
                  sort={controls.sort}
                  onSort={controls.setSort}
                  options={[
                    { key: "vlan_id", label: "VLAN ID" },
                    { key: "name", label: "Name" },
                    { key: "description", label: "Description" },
                  ]}
                />
                <ItemGroup>
                  {sortedItems.map((vlan) => (
                    <Item
                      key={vlan.vlan_id}
                      role="listitem"
                      variant="outline"
                      size="sm"
                    >
                      <ItemContent className="min-w-0">
                        <ItemTitle className="max-w-full min-w-0 flex-wrap wrap-anywhere">
                          <VlanBadge name={vlan.name} color={vlan.color} />
                          VLAN {vlan.vlan_id}
                        </ItemTitle>
                        {vlan.description ? (
                          <ItemDescription className="wrap-anywhere">
                            {vlan.description}
                          </ItemDescription>
                        ) : null}
                      </ItemContent>
                      <ItemFooter>
                        {deviceCount(vlan)}
                        {vlanActions(vlan)}
                      </ItemFooter>
                    </Item>
                  ))}
                </ItemGroup>
              </ListMobile>
            </>
          )}
        </CardContent>
      </Card>

      <GuardedDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        dirty={JSON.stringify(form) !== JSON.stringify(initialForm)}
        busy={submitting}
      >
        <DialogContent
          showCloseButton={!submitting}
          className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl"
        >
          <DialogHeader>
            <DialogTitle>{editing ? "Edit VLAN" : "Create VLAN"}</DialogTitle>
            <DialogDescription>
              {editing
                ? "Update the VLAN definition."
                : "Add a new VLAN definition for device assignment."}
            </DialogDescription>
          </DialogHeader>
          <form className="flex min-h-0 flex-col gap-6" onSubmit={handleSubmit}>
            <fieldset disabled={submitting} className="min-w-0">
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="vlan-name">Name</FieldLabel>
                  <Input
                    id="vlan-name"
                    value={form.name}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        name: event.target.value,
                      }))
                    }
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="vlan-id">VLAN ID</FieldLabel>
                  <Input
                    id="vlan-id"
                    type="number"
                    min={1}
                    max={4094}
                    value={form.vlan_id}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        vlan_id: event.target.value,
                      }))
                    }
                    required
                  />
                  <FieldDescription>
                    Valid range is 1 through 4094.
                  </FieldDescription>
                </Field>
                <Field>
                  <FieldLabel htmlFor="vlan-description">
                    Description
                  </FieldLabel>
                  <Textarea
                    id="vlan-description"
                    value={form.description}
                    onChange={(event) =>
                      setForm((current) => ({
                        ...current,
                        description: event.target.value,
                      }))
                    }
                  />
                </Field>
                <Field data-disabled={submitting}>
                  <FieldTitle id="vlan-color-label">Color</FieldTitle>
                  <ToggleGroup
                    aria-labelledby="vlan-color-label"
                    variant="outline"
                    size="sm"
                    spacing={2}
                    className="flex-wrap"
                    value={[form.color]}
                    onValueChange={(values) => {
                      const color = values[0] as VlanColor | undefined
                      if (color) {
                        setForm((current) => ({ ...current, color }))
                      }
                    }}
                    disabled={submitting}
                  >
                    {vlanColors.map(({ value, label }) => (
                      <ToggleGroupItem
                        key={value}
                        value={value}
                        aria-label={label}
                        title={label}
                        type="button"
                      >
                        <span
                          className="size-4 rounded-full"
                          data-vlan-swatch
                          data-vlan-color={value}
                          aria-hidden="true"
                        />
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </Field>
                <FieldError>{submitError}</FieldError>
              </FieldGroup>
            </fieldset>
            <DialogFooter>
              <DialogClose
                render={
                  <Button
                    type="button"
                    variant="outline"
                    disabled={submitting}
                  />
                }
              >
                Cancel
              </DialogClose>
              <Button type="submit" disabled={submitting}>
                {submitting
                  ? "Saving..."
                  : editing
                    ? "Save changes"
                    : "Create VLAN"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </GuardedDialog>

      <AlertDialog
        open={Boolean(deleting)}
        onOpenChange={(open) => !open && !deletingBusy && setDeleting(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete VLAN</AlertDialogTitle>
            <AlertDialogDescription>
              {deleting
                ? `Delete ${deleting.name} (VLAN ${deleting.vlan_id})? This removes the VLAN definition.`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingBusy}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deletingBusy}
              onClick={handleDelete}
            >
              {deletingBusy ? "Deleting..." : "Delete VLAN"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
