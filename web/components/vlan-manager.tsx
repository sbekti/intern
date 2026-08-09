"use client"

import { useRouter } from "next/navigation"
import { startTransition, useMemo, useState } from "react"
import { toast } from "sonner"
import { NetworkIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"

import type { Vlan } from "@/lib/api"
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
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"

type VlanFormState = {
  name: string
  vlan_id: string
  description: string
}

type ApiError = {
  code: string
  message: string
}

const defaultFormState: VlanFormState = {
  name: "",
  vlan_id: "",
  description: "",
}

function mapVlanToForm(vlan: Vlan): VlanFormState {
  return {
    name: vlan.name,
    vlan_id: String(vlan.vlan_id),
    description: vlan.description,
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

export function VlanManager({
  initialItems,
}: {
  initialItems: Vlan[]
}) {
  const router = useRouter()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Vlan | null>(null)
  const [deleting, setDeleting] = useState<Vlan | null>(null)
  const [form, setForm] = useState<VlanFormState>(defaultFormState)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [deletingBusy, setDeletingBusy] = useState(false)

  const sortedItems = useMemo(
    () => [...initialItems].sort((a, b) => a.vlan_id - b.vlan_id),
    [initialItems]
  )

  function openCreate() {
    setEditing(null)
    setForm(defaultFormState)
    setSubmitError(null)
    setDialogOpen(true)
  }

  function openEdit(vlan: Vlan) {
    setEditing(vlan)
    setForm(mapVlanToForm(vlan))
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
    }

    const response = await fetch(
      editing
        ? buildBffPath(`/networks/vlans/${editing.vlan_id}`)
        : buildBffPath("/networks/vlans"),
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

    toast.success(editing ? "VLAN updated" : "VLAN created")
    setDialogOpen(false)
    setSubmitting(false)
    startTransition(() => router.refresh())
  }

  async function handleDelete() {
    if (!deleting) {
      return
    }

    setDeletingBusy(true)

    const response = await fetch(buildBffPath(`/networks/vlans/${deleting.vlan_id}`), {
      method: "DELETE",
    })

    if (!response.ok) {
      setDeletingBusy(false)
      toast.error(await parseApiError(response))
      return
    }

    toast.success("VLAN deleted")
    setDeleting(null)
    setDeletingBusy(false)
    startTransition(() => router.refresh())
  }

  return (
    <>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <div className="flex flex-col gap-1">
            <CardTitle>VLANs</CardTitle>
            <CardDescription>
              Create, update, and remove VLAN definitions for the network.
            </CardDescription>
          </div>
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
        <CardContent>
          {sortedItems.length === 0 ? (
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
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>VLAN ID</TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedItems.map((vlan) => (
                  <TableRow key={vlan.vlan_id}>
                    <TableCell>{vlan.vlan_id}</TableCell>
                    <TableCell className="font-medium">{vlan.name}</TableCell>
                    <TableCell className="max-w-[24rem] whitespace-normal text-muted-foreground">
                      {vlan.description || "-"}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="icon-sm"
                          className={iconOnlyButtonClass}
                          onClick={() => openEdit(vlan)}
                          aria-label="Edit VLAN"
                        >
                          <PencilIcon data-icon="inline-start" />
                          <IconOnlyButtonLabel>Edit</IconOnlyButtonLabel>
                        </Button>
                        <Button
                          variant="outline"
                          size="icon-sm"
                          className={responsiveCompactButtonClass}
                          onClick={() => setDeleting(vlan)}
                          aria-label="Delete VLAN"
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
            <DialogTitle>{editing ? "Edit VLAN" : "Create VLAN"}</DialogTitle>
            <DialogDescription>
              {editing
                ? "Update the VLAN definition."
                : "Add a new VLAN definition for device assignment."}
            </DialogDescription>
          </DialogHeader>
          <form className="grid gap-6" onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="vlan-name">Name</FieldLabel>
                <Input
                  id="vlan-name"
                  value={form.name}
                  onChange={(event) =>
                    setForm((current) => ({ ...current, name: event.target.value }))
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
                <FieldLabel htmlFor="vlan-description">Description</FieldLabel>
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
                {submitting ? "Saving..." : editing ? "Save changes" : "Create VLAN"}
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
            <AlertDialogTitle>Delete VLAN</AlertDialogTitle>
            <AlertDialogDescription>
              {deleting
                ? `Delete ${deleting.name} (VLAN ${deleting.vlan_id})? This will remove the definition from the backend.`
                : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingBusy}>Cancel</AlertDialogCancel>
            <AlertDialogAction disabled={deletingBusy} onClick={handleDelete}>
              {deletingBusy ? "Deleting..." : "Delete VLAN"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
