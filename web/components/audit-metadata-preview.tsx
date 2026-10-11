"use client"

import { CopyIcon } from "lucide-react"
import { toast } from "@/components/ui/toast"

import { Button } from "@/components/ui/button"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"

function formatMetadata(metadata: Record<string, unknown>) {
  try {
    return {
      preview: JSON.stringify(metadata),
      full: JSON.stringify(metadata, null, 2),
    }
  } catch {
    const fallback = String(metadata)
    return {
      preview: fallback,
      full: fallback,
    }
  }
}

function MetadataPreviewCode({ preview }: { preview: string }) {
  return (
    <code className="line-clamp-2 text-xs break-all text-muted-foreground">
      {preview}
    </code>
  )
}

function MetadataDetail({
  full,
  onCopy,
}: {
  full: string
  onCopy: () => void
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs font-semibold">Metadata</p>
        <Button variant="ghost" size="xs" onClick={onCopy}>
          <CopyIcon data-icon="inline-start" />
          Copy
        </Button>
      </div>
      <pre className="max-h-72 overflow-auto rounded-md bg-muted/40 p-2 text-xs leading-5 text-foreground">
        {full}
      </pre>
    </div>
  )
}

export function AuditMetadataPreview({
  metadata,
  actorUsername,
  action,
  compact = false,
}: {
  compact?: boolean
  metadata: Record<string, unknown>
  actorUsername: string
  action: string
}) {
  const isMobile = useIsMobile()
  const formatted = formatMetadata(metadata)

  async function copyMetadata() {
    try {
      await navigator.clipboard.writeText(formatted.full)
      toast.add({ type: "success", title: "Metadata copied." })
    } catch {
      toast.add({ type: "error", title: "Metadata copy failed." })
    }
  }

  const trigger = (
    <Button
      variant="ghost"
      size="sm"
      className="h-auto min-h-8 max-w-full min-w-0 justify-start whitespace-normal"
      aria-label={`View details for ${action} by ${actorUsername || "unknown actor"}`}
    >
      {compact ? (
        "View details"
      ) : (
        <MetadataPreviewCode preview={formatted.preview} />
      )}
    </Button>
  )

  if (isMobile) {
    return (
      <Drawer>
        <DrawerTrigger asChild>{trigger}</DrawerTrigger>
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>Audit details</DrawerTitle>
            <DrawerDescription className="wrap-anywhere">
              {actorUsername || "Unknown actor"} · {action}
            </DrawerDescription>
          </DrawerHeader>
          <div className="min-h-0 overflow-y-auto px-4 pb-4">
            <MetadataDetail full={formatted.full} onCopy={copyMetadata} />
          </div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <Dialog>
      <DialogTrigger render={trigger} />
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Audit details</DialogTitle>
          <DialogDescription className="wrap-anywhere">
            {actorUsername || "Unknown actor"} · {action}
          </DialogDescription>
        </DialogHeader>
        <MetadataDetail full={formatted.full} onCopy={copyMetadata} />
      </DialogContent>
    </Dialog>
  )
}
