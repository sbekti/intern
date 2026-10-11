"use client"

import Link from "next/link"
import {
  CopyIcon,
  EllipsisIcon,
  PencilIcon,
  ScrollTextIcon,
  Trash2Icon,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { toast } from "@/components/ui/toast"

export function RecordActions({
  name,
  onEdit,
  onDelete,
  activityHref,
  copies = [],
  deleteLabel = "Delete",
}: {
  name: string
  onEdit: () => void
  onDelete: () => void
  activityHref: string
  copies?: { label: string; value: string }[]
  deleteLabel?: string
}) {
  return (
    <div className="flex shrink-0 items-center justify-end gap-2">
      <Button
        variant="outline"
        size="sm"
        onClick={onEdit}
        aria-label={`Edit ${name}`}
      >
        <PencilIcon data-icon="inline-start" />
        Edit
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={`More actions for ${name}`}
            />
          }
        >
          <EllipsisIcon />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuGroup>
            {copies.map(({ label, value }) => (
              <DropdownMenuItem
                key={label}
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(value)
                    toast.add({ type: "success", title: `${label} copied` })
                  } catch {
                    toast.add({
                      type: "error",
                      title: "Couldn't copy. Try again.",
                    })
                  }
                }}
              >
                <CopyIcon />
                Copy {label}
              </DropdownMenuItem>
            ))}
            <DropdownMenuItem render={<Link href={activityHref} />}>
              <ScrollTextIcon />
              View activity
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              <Trash2Icon />
              {deleteLabel}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
