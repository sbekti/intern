"use client"

import { useRouter } from "next/navigation"
import { startTransition, useMemo, useState } from "react"
import { toast } from "@/components/ui/toast"
import {
  MonitorSmartphoneIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "lucide-react"

import type { NetworkDevice, Vlan } from "@/lib/api"
import { VlanBadge } from "@/components/vlan-badge"
import { useListControls } from "@/hooks/use-list-controls"
import { filterAndSort } from "@/lib/list-controls"
import {
  deviceFilterFields,
  deviceFilterValues,
  deviceSortValues,
} from "@/lib/network-list"
import {
  NoMatchingRecords,
  TokenFilterBar,
} from "@/components/token-filter-bar"
import { SortableTableHead } from "@/components/sortable-table-head"
import { buildBffPath } from "@/lib/bff"
import {
  IconOnlyButtonLabel,
  responsiveCompactButtonClass,
  iconOnlyButtonClass,
} from "@/components/compact-button-label"
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
  Dialog,
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
  FieldContent,
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLegend,
  FieldLabel,
  FieldSet,
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
import { Checkbox } from "@/components/ui/checkbox"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type DeviceFormState = {
  display_name: string
  mac_address: string
  disabled: boolean
  vlan_id: string
}

type ApiError = {
  code: string
  message: string
}

const defaultFormState: DeviceFormState = {
  display_name: "",
  mac_address: "",
  disabled: false,
  vlan_id: "",
}

const deviceDisabledFieldId = "device-disabled"

function mapDeviceToForm(device: NetworkDevice): DeviceFormState {
  return {
    display_name: device.display_name,
    mac_address: device.mac_address,
    disabled: device.disabled,
    vlan_id: String(device.vlan.vlan_id),
  }
}

async function parseApiError(response: Response) {
  try {
    const body = (await response.json()) as Partial<ApiError>
    return body.message ?? `${response.status} ${response.statusText}`
  } catch {
    return `${response.status} ${response.statusText}`
  }
}

export function DeviceManager({
  initialItems,
  vlans,
}: {
  initialItems: NetworkDevice[]
  vlans: Vlan[]
}) {
  const router = useRouter()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<NetworkDevice | null>(null)
  const [deleting, setDeleting] = useState<NetworkDevice | null>(null)
  const [form, setForm] = useState<DeviceFormState>(defaultFormState)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [deletingBusy, setDeletingBusy] = useState(false)

  const controls = useListControls(
    Object.keys(deviceFilterValues),
    Object.keys(deviceSortValues),
    { key: "display_name", direction: "asc" }
  )
  const sortedItems = filterAndSort(
    initialItems,
    controls.filters,
    deviceFilterValues,
    controls.sort,
    deviceSortValues,
    (item) => item.id
  )
  const filterFields = deviceFilterFields(initialItems, vlans)

  const sortedVlans = useMemo(
    () => [...vlans].sort((a, b) => a.vlan_id - b.vlan_id),
    [vlans]
  )

  function openCreate() {
    setEditing(null)
    setForm({
      ...defaultFormState,
      vlan_id: sortedVlans[0] ? String(sortedVlans[0].vlan_id) : "",
    })
    setSubmitError(null)
    setDialogOpen(true)
  }

  function openEdit(device: NetworkDevice) {
    setEditing(device)
    setForm(mapDeviceToForm(device))
    setSubmitError(null)
    setDialogOpen(true)
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setSubmitError(null)

    const payload = {
      display_name: form.display_name.trim(),
      mac_address: form.mac_address.trim(),
      disabled: form.disabled,
      vlan_id: Number(form.vlan_id),
    }

    const response = await fetch(
      editing
        ? buildBffPath(`/networks/devices/${editing.id}`)
        : buildBffPath("/networks/devices"),
      {
        method: editing ? "PATCH" : "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify(payload),
      }
    )

    if (!response.ok) {
      setSubmitting(false)
      setSubmitError(await parseApiError(response))
      return
    }

    toast.add({ type: "success", title: editing ? "Device updated" : "Device created" })
    setDialogOpen(false)
    setSubmitting(false)
    startTransition(() => router.refresh())
  }

  async function handleDelete() {
    if (!deleting) {
      return
    }

    setDeletingBusy(true)

    const response = await fetch(
      buildBffPath(`/networks/devices/${deleting.id}`),
      {
        method: "DELETE",
      }
    )

    if (!response.ok) {
      setDeletingBusy(false)
      toast.add({ type: "error", title: await parseApiError(response) })
      return
    }

    toast.add({ type: "success", title: "Device deleted" })
    setDeleting(null)
    setDeletingBusy(false)
    startTransition(() => router.refresh())
  }

  return (
    <>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>Devices</CardTitle>
          <CardDescription>
            Register devices and assign each MAC address to the correct VLAN.
          </CardDescription>
          <CardAction className="ml-3 sm:ml-0">
            <Button
              size="icon-sm"
              className={responsiveCompactButtonClass}
              onClick={openCreate}
              disabled={sortedVlans.length === 0}
              aria-label="Add Device"
            >
              <PlusIcon data-icon="inline-start" />
              <span className="hidden sm:inline">Add Device</span>
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
          ) : sortedVlans.length === 0 ? (
            <Empty className="min-h-[16rem] border bg-muted/20">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <MonitorSmartphoneIcon />
                </EmptyMedia>
                <EmptyTitle>No VLANs available</EmptyTitle>
                <EmptyDescription>
                  Create at least one VLAN before registering devices.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : sortedItems.length === 0 ? (
            <Empty className="min-h-[16rem] border bg-muted/20">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <MonitorSmartphoneIcon />
                </EmptyMedia>
                <EmptyTitle>No devices</EmptyTitle>
                <EmptyDescription>
                  Register the first device to assign it to a VLAN.
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={openCreate}>
                  <PlusIcon data-icon="inline-start" />
                  Add Device
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
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
                    sortKey="vlan"
                    sort={controls.sort}
                    onSort={controls.setSort}
                  >
                    VLAN
                  </SortableTableHead>
                  <SortableTableHead
                    sortKey="status"
                    sort={controls.sort}
                    onSort={controls.setSort}
                  >
                    Status
                  </SortableTableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedItems.map((device) => (
                  <TableRow key={device.id}>
                    <TableCell className="font-medium">
                      {device.display_name}
                    </TableCell>
                    <TableCell className="font-mono">
                      {device.mac_address}
                    </TableCell>
                    <TableCell>
                      <VlanBadge
                        name={device.vlan.name}
                        color={device.vlan.color}
                      />
                    </TableCell>
                    <TableCell>
                      {device.disabled ? "Disabled" : "Enabled"}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="icon-sm"
                          className={iconOnlyButtonClass}
                          onClick={() => openEdit(device)}
                          aria-label="Edit device"
                        >
                          <PencilIcon data-icon="inline-start" />
                          <IconOnlyButtonLabel>Edit</IconOnlyButtonLabel>
                        </Button>
                        <Button
                          variant="outline"
                          size="icon-sm"
                          className={responsiveCompactButtonClass}
                          onClick={() => setDeleting(device)}
                          aria-label="Delete device"
                        >
                          <Trash2Icon data-icon="inline-start" />
                          <IconOnlyButtonLabel>Delete</IconOnlyButtonLabel>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {editing ? "Edit device" : "Create device"}
            </DialogTitle>
            <DialogDescription>
              {editing
                ? "Update the device name, MAC address, VLAN assignment, or authentication state."
                : "Register a MAC address, attach it to a VLAN, and choose whether it should authenticate through RADIUS."}
            </DialogDescription>
          </DialogHeader>
          <form className="grid gap-6" onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="device-name">Display name</FieldLabel>
                <Input
                  id="device-name"
                  value={form.display_name}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      display_name: event.target.value,
                    }))
                  }
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="device-mac">MAC address</FieldLabel>
                <Input
                  id="device-mac"
                  value={form.mac_address}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      mac_address: event.target.value,
                    }))
                  }
                  required
                />
                <FieldDescription>
                  Accepted formats include colon, hyphen, dotted, or bare
                  hexadecimal.
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="device-vlan">VLAN</FieldLabel>
                <Select
                  value={form.vlan_id}
                  onValueChange={(value) =>
                    setForm((current) => ({
                      ...current,
                      vlan_id: value ?? "",
                    }))
                  }
                  items={sortedVlans.map((vlan) => ({
                    label: vlan.name,
                    value: String(vlan.vlan_id),
                  }))}
                >
                  <SelectTrigger id="device-vlan" className="w-full">
                    <SelectValue placeholder="Select a VLAN" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {sortedVlans.map((vlan) => (
                        <SelectItem
                          key={vlan.vlan_id}
                          value={String(vlan.vlan_id)}
                        >
                          {vlan.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <FieldSet>
                <FieldLegend variant="label">Authentication</FieldLegend>
                <Field orientation="horizontal" className="items-start">
                  <Checkbox
                    id={deviceDisabledFieldId}
                    checked={form.disabled}
                    onCheckedChange={(checked) =>
                      setForm((current) => ({
                        ...current,
                        disabled: checked,
                      }))
                    }
                  />
                  <FieldContent>
                    <FieldLabel htmlFor={deviceDisabledFieldId}>
                      Disable this device
                    </FieldLabel>
                    <FieldDescription>
                      Disabled devices stay visible and keep their MAC address,
                      but they are excluded from RADIUS authentication until
                      they are enabled again.
                    </FieldDescription>
                  </FieldContent>
                </Field>
              </FieldSet>
              <FieldError>{submitError}</FieldError>
            </FieldGroup>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setDialogOpen(false)}
                disabled={submitting}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting
                  ? "Saving..."
                  : editing
                    ? "Save changes"
                    : "Create device"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleting)}
        onOpenChange={(open) => !open && setDeleting(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete device</AlertDialogTitle>
            <AlertDialogDescription>
              {deleting
                ? `Delete ${deleting.display_name} (${deleting.mac_address}) from device management?`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingBusy}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction disabled={deletingBusy} onClick={handleDelete}>
              {deletingBusy ? "Deleting..." : "Delete device"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
