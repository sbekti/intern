"use client"

import { ListTable, ListMobile } from "@/components/responsive-list"

import Link from "next/link"
import { startTransition, useMemo, useState } from "react"
import { useRouter } from "next/navigation"
import { PlusIcon, TabletSmartphoneIcon } from "lucide-react"

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
import type { Gizmo, NetworkDevice } from "@/lib/api"
import { useListControls } from "@/hooks/use-list-controls"
import { filterAndSort } from "@/lib/list-controls"
import {
  gizmoFilterFields,
  gizmoFilterValues,
  gizmoSortValues,
} from "@/lib/network-list"
import {
  NoMatchingRecords,
  TokenFilterBar,
} from "@/components/token-filter-bar"
import { SortableTableHead } from "@/components/sortable-table-head"
import { buildBffPath } from "@/lib/bff"
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
import { Badge } from "@/components/ui/badge"
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
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { toast } from "@/components/ui/toast"

export function GizmoManager({
  initialItems,
  devices,
}: {
  initialItems: Gizmo[]
  devices: NetworkDevice[]
}) {
  const router = useRouter()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Gizmo | null>(null)
  const [removing, setRemoving] = useState<Gizmo | null>(null)
  const [deviceId, setDeviceId] = useState("")
  const [kioskUrl, setKioskUrl] = useState("")
  const [initialForm, setInitialForm] = useState({ deviceId: "", kioskUrl: "" })
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [removingBusy, setRemovingBusy] = useState(false)

  const controls = useListControls(
    Object.keys(gizmoFilterValues),
    Object.keys(gizmoSortValues),
    { key: "display_name", direction: "asc" }
  )
  const sortedItems = filterAndSort(
    initialItems,
    controls.filters,
    gizmoFilterValues,
    controls.sort,
    gizmoSortValues,
    (item) => item.network_device.id
  )
  const filterFields = gizmoFilterFields(initialItems)

  const availableDevices = useMemo(() => {
    const assigned = new Set(initialItems.map((item) => item.network_device.id))
    return devices
      .filter((device) => !assigned.has(device.id))
      .sort((a, b) =>
        a.display_name.localeCompare(b.display_name, undefined, {
          sensitivity: "base",
        })
      )
  }, [devices, initialItems])

  function openCreate() {
    setEditing(null)
    setDeviceId(availableDevices[0]?.id ?? "")
    setKioskUrl("")
    setInitialForm({ deviceId: availableDevices[0]?.id ?? "", kioskUrl: "" })
    setSubmitError(null)
    setDialogOpen(true)
  }

  function openEdit(item: Gizmo) {
    setEditing(item)
    setDeviceId(item.network_device.id)
    setKioskUrl(item.kiosk_url ?? "")
    setInitialForm({
      deviceId: item.network_device.id,
      kioskUrl: item.kiosk_url ?? "",
    })
    setSubmitError(null)
    setDialogOpen(true)
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setSubmitError(null)

    try {
      await mutate(
        editing ? buildBffPath(`/gizmos/${deviceId}`) : buildBffPath("/gizmos"),
        {
          method: editing ? "PUT" : "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify(
            editing
              ? { kiosk_url: kioskUrl }
              : { network_device_id: deviceId, kiosk_url: kioskUrl }
          ),
        }
      )
      toast.add({
        type: "success",
        title: editing ? "Gizmo updated" : "Gizmo created",
      })
      setDialogOpen(false)
      startTransition(() => router.refresh())
    } catch (error) {
      setSubmitError(
        error instanceof Error
          ? error.message
          : "Couldn't save the Gizmo. Try again."
      )
    } finally {
      setSubmitting(false)
    }
  }

  async function handleRemove() {
    if (!removing) return
    setRemovingBusy(true)
    try {
      await mutate(buildBffPath(`/gizmos/${removing.network_device.id}`), {
        method: "DELETE",
      })
      toast.add({ type: "success", title: "Gizmo removed" })
      setRemoving(null)
      startTransition(() => router.refresh())
    } catch (error) {
      toast.add({
        type: "error",
        title:
          error instanceof Error ? error.message : "Couldn't remove the Gizmo.",
      })
    } finally {
      setRemovingBusy(false)
    }
  }

  function gizmoActions(item: Gizmo) {
    return (
      <RecordActions
        name={item.network_device.display_name}
        onEdit={() => openEdit(item)}
        onDelete={() => setRemoving(item)}
        deleteLabel="Remove"
        activityHref={`/admin/audit-logs?resource_type=gizmo&resource_id=${encodeURIComponent(item.network_device.id)}`}
        copies={[
          { label: "MAC address", value: item.network_device.mac_address },
          ...(item.kiosk_url
            ? [{ label: "kiosk URL", value: item.kiosk_url }]
            : []),
        ]}
      />
    )
  }
  function deviceLink(item: Gizmo) {
    return (
      <Link
        className="underline underline-offset-4"
        href={`/networks/devices?mac_address=${encodeURIComponent(item.network_device.mac_address)}`}
      >
        {item.network_device.display_name}
      </Link>
    )
  }

  return (
    <>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>Gizmos</CardTitle>
          <CardDescription>
            {sortedItems.length} of {initialItems.length} gizmos
          </CardDescription>
          <CardAction className="ml-3 sm:ml-0">
            <Button
              size="icon-sm"
              className={responsiveCompactButtonClass}
              onClick={openCreate}
              disabled={availableDevices.length === 0}
              aria-label="Add Gizmo"
            >
              <PlusIcon data-icon="inline-start" />
              <span className="hidden sm:inline">Add Gizmo</span>
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
                  <TabletSmartphoneIcon />
                </EmptyMedia>
                <EmptyTitle>No Gizmos</EmptyTitle>
                <EmptyDescription>
                  Register an existing network device as the first Gizmo.
                </EmptyDescription>
              </EmptyHeader>
              {availableDevices.length > 0 ? (
                <EmptyContent>
                  <Button onClick={openCreate}>
                    <PlusIcon data-icon="inline-start" />
                    Add Gizmo
                  </Button>
                </EmptyContent>
              ) : null}
            </Empty>
          ) : (
            <>
              <ListTable>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <SortableTableHead
                        sortKey="display_name"
                        sort={controls.sort}
                        onSort={controls.setSort}
                      >
                        Name
                      </SortableTableHead>
                      <SortableTableHead
                        sortKey="mac_address"
                        sort={controls.sort}
                        onSort={controls.setSort}
                      >
                        MAC Address
                      </SortableTableHead>
                      <SortableTableHead
                        sortKey="destination"
                        sort={controls.sort}
                        onSort={controls.setSort}
                      >
                        Destination
                      </SortableTableHead>
                      <TableHead className="text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {sortedItems.map((item) => (
                      <TableRow key={item.network_device.id}>
                        <TableCell className="font-medium">
                          {deviceLink(item)}
                        </TableCell>
                        <TableCell className="font-mono">
                          {item.network_device.mac_address}
                        </TableCell>
                        <TableCell className="max-w-80">
                          {item.kiosk_url ? (
                            <span
                              className="block truncate"
                              title={item.kiosk_url}
                            >
                              {item.kiosk_url}
                            </span>
                          ) : (
                            <Badge variant="secondary">Unconfigured</Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-right">
                          {gizmoActions(item)}
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
                    { key: "display_name", label: "Name" },
                    { key: "mac_address", label: "MAC address" },
                    { key: "destination", label: "Destination" },
                  ]}
                />
                <ItemGroup>
                  {sortedItems.map((item) => (
                    <Item
                      key={item.network_device.id}
                      role="listitem"
                      variant="outline"
                      size="sm"
                    >
                      <ItemContent className="min-w-0">
                        <ItemTitle className="max-w-full min-w-0 wrap-anywhere">
                          {deviceLink(item)}
                        </ItemTitle>
                        <ItemDescription>
                          {item.network_device.mac_address}
                        </ItemDescription>
                        {item.kiosk_url ? (
                          <ItemDescription className="wrap-anywhere">
                            {item.kiosk_url}
                          </ItemDescription>
                        ) : (
                          <Badge variant="secondary" className="self-start">
                            Unconfigured
                          </Badge>
                        )}
                      </ItemContent>
                      <ItemFooter className="justify-end">
                        {gizmoActions(item)}
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
        dirty={
          deviceId !== initialForm.deviceId || kioskUrl !== initialForm.kioskUrl
        }
        busy={submitting}
      >
        <DialogContent
          showCloseButton={!submitting}
          className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl"
        >
          <DialogHeader>
            <DialogTitle>{editing ? "Edit Gizmo" : "Add Gizmo"}</DialogTitle>
            <DialogDescription>
              {editing
                ? `Update the kiosk destination for ${editing.network_device.display_name}.`
                : "Choose a registered device and optionally assign its kiosk destination."}
            </DialogDescription>
          </DialogHeader>
          <form className="flex min-h-0 flex-col gap-6" onSubmit={handleSubmit}>
            <fieldset disabled={submitting} className="min-w-0">
              <FieldGroup>
                {!editing ? (
                  <Field>
                    <FieldLabel htmlFor="gizmo-device">
                      Network device
                    </FieldLabel>
                    <Select
                      disabled={submitting}
                      value={deviceId}
                      onValueChange={(value) => setDeviceId(value ?? "")}
                      items={availableDevices.map((device) => ({
                        label: `${device.display_name} (${device.mac_address})`,
                        value: device.id,
                      }))}
                    >
                      <SelectTrigger id="gizmo-device" className="w-full">
                        <SelectValue placeholder="Select a device" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {availableDevices.map((device) => (
                            <SelectItem key={device.id} value={device.id}>
                              {device.display_name} ({device.mac_address})
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                ) : null}
                <Field>
                  <FieldLabel htmlFor="gizmo-url">Kiosk URL</FieldLabel>
                  <Input
                    id="gizmo-url"
                    inputMode="url"
                    maxLength={2048}
                    value={kioskUrl}
                    onChange={(event) => setKioskUrl(event.target.value)}
                    placeholder="https://ha.example.com/dashboard"
                  />
                  <FieldDescription>
                    Leave empty to keep the Gizmo registered without a
                    destination.
                  </FieldDescription>
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
              <Button type="submit" disabled={submitting || !deviceId}>
                {submitting
                  ? "Saving..."
                  : editing
                    ? "Save changes"
                    : "Add Gizmo"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </GuardedDialog>

      <AlertDialog
        open={Boolean(removing)}
        onOpenChange={(open) => !open && !removingBusy && setRemoving(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove Gizmo</AlertDialogTitle>
            <AlertDialogDescription>
              {removing
                ? `Remove ${removing.network_device.display_name} from Gizmos? Its network-device registration and VLAN assignment will remain.`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removingBusy}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={removingBusy}
              onClick={handleRemove}
            >
              {removingBusy ? "Removing..." : "Remove Gizmo"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
