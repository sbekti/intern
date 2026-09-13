"use client"

import { startTransition, useMemo, useState } from "react"
import { useRouter } from "next/navigation"
import {
  PencilIcon,
  PlusIcon,
  TabletSmartphoneIcon,
  Trash2Icon,
} from "lucide-react"

import type { Gizmo, NetworkDevice } from "@/lib/api"
import { buildBffPath } from "@/lib/bff"
import {
  IconOnlyButtonLabel,
  iconOnlyButtonClass,
  responsiveCompactButtonClass,
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

type ApiError = { message?: string }

async function parseApiError(response: Response) {
  try {
    const body = (await response.json()) as ApiError
    return body.message ?? `${response.status} ${response.statusText}`
  } catch {
    return `${response.status} ${response.statusText}`
  }
}

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
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [removingBusy, setRemovingBusy] = useState(false)

  const sortedItems = useMemo(
    () =>
      [...initialItems].sort((a, b) =>
        a.network_device.display_name.localeCompare(
          b.network_device.display_name,
          undefined,
          { sensitivity: "base" }
        )
      ),
    [initialItems]
  )
  const availableDevices = useMemo(() => {
    const assigned = new Set(
      initialItems.map((item) => item.network_device.id)
    )
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
    setSubmitError(null)
    setDialogOpen(true)
  }

  function openEdit(item: Gizmo) {
    setEditing(item)
    setDeviceId(item.network_device.id)
    setKioskUrl(item.kiosk_url ?? "")
    setSubmitError(null)
    setDialogOpen(true)
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setSubmitError(null)

    const response = await fetch(
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

    if (!response.ok) {
      setSubmitting(false)
      setSubmitError(await parseApiError(response))
      return
    }

    toast.add({
      type: "success",
      title: editing ? "Gizmo updated" : "Gizmo created",
    })
    setDialogOpen(false)
    setSubmitting(false)
    startTransition(() => router.refresh())
  }

  async function handleRemove() {
    if (!removing) return
    setRemovingBusy(true)
    const response = await fetch(
      buildBffPath(`/gizmos/${removing.network_device.id}`),
      { method: "DELETE" }
    )
    if (!response.ok) {
      setRemovingBusy(false)
      toast.add({ type: "error", title: await parseApiError(response) })
      return
    }
    toast.add({ type: "success", title: "Gizmo removed" })
    setRemoving(null)
    setRemovingBusy(false)
    startTransition(() => router.refresh())
  }

  return (
    <>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <div className="flex flex-col gap-1">
            <CardTitle>Gizmos</CardTitle>
            <CardDescription>
              Assign kiosk destinations to registered network devices.
            </CardDescription>
          </div>
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
        <CardContent>
          {sortedItems.length === 0 ? (
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
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>MAC Address</TableHead>
                  <TableHead>Destination</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedItems.map((item) => (
                  <TableRow key={item.network_device.id}>
                    <TableCell className="font-medium">
                      {item.network_device.display_name}
                    </TableCell>
                    <TableCell className="font-mono">
                      {item.network_device.mac_address}
                    </TableCell>
                    <TableCell className="max-w-80">
                      {item.kiosk_url ? (
                        <span className="block truncate" title={item.kiosk_url}>
                          {item.kiosk_url}
                        </span>
                      ) : (
                        <Badge variant="secondary">Unconfigured</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="icon-sm"
                          className={iconOnlyButtonClass}
                          onClick={() => openEdit(item)}
                          aria-label="Edit Gizmo"
                        >
                          <PencilIcon data-icon="inline-start" />
                          <IconOnlyButtonLabel>Edit</IconOnlyButtonLabel>
                        </Button>
                        <Button
                          variant="outline"
                          size="icon-sm"
                          className={responsiveCompactButtonClass}
                          onClick={() => setRemoving(item)}
                          aria-label="Remove Gizmo"
                        >
                          <Trash2Icon data-icon="inline-start" />
                          <IconOnlyButtonLabel>Remove</IconOnlyButtonLabel>
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
            <DialogTitle>{editing ? "Edit Gizmo" : "Add Gizmo"}</DialogTitle>
            <DialogDescription>
              {editing
                ? `Update the kiosk destination for ${editing.network_device.display_name}.`
                : "Choose a registered device and optionally assign its kiosk destination."}
            </DialogDescription>
          </DialogHeader>
          <form className="grid gap-6" onSubmit={handleSubmit}>
            <FieldGroup>
              {!editing ? (
                <Field>
                  <FieldLabel htmlFor="gizmo-device">Network device</FieldLabel>
                  <Select
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
                  Leave empty to keep the Gizmo registered without a destination.
                </FieldDescription>
              </Field>
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
              <Button type="submit" disabled={submitting || !deviceId}>
                {submitting ? "Saving..." : editing ? "Save changes" : "Add Gizmo"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(removing)}
        onOpenChange={(open) => !open && setRemoving(null)}
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
            <AlertDialogCancel disabled={removingBusy}>Cancel</AlertDialogCancel>
            <AlertDialogAction disabled={removingBusy} onClick={handleRemove}>
              {removingBusy ? "Removing..." : "Remove Gizmo"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
