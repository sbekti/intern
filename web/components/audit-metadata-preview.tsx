"use client"

import { CopyIcon } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  hoverPreviewCloseDelay,
  hoverPreviewDelay,
} from "@/lib/hover"
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer"
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "@/components/ui/hover-card"

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

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }

  const textarea = document.createElement("textarea")
  textarea.value = value
  textarea.setAttribute("readonly", "")
  textarea.style.position = "fixed"
  textarea.style.opacity = "0"
  document.body.appendChild(textarea)
  textarea.select()

  try {
    const copied = document.execCommand("copy")
    if (!copied) {
      throw new Error("copy command failed")
    }
  } finally {
    document.body.removeChild(textarea)
  }
}

function MetadataPreviewCode({ preview }: { preview: string }) {
  return (
    <code className="line-clamp-2 break-all text-xs text-muted-foreground">
      {preview}
    </code>
  )
}

function MetadataDetail({
  full,
  onCopy,
  showTitle = true,
}: {
  full: string
  onCopy: () => void
  showTitle?: boolean
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between gap-2">
        {showTitle ? <p className="text-xs font-semibold">Metadata</p> : <span />}
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

function MetadataContext({
  actorUsername,
  action,
  showTitle = true,
}: {
  actorUsername: string
  action: string
  showTitle?: boolean
}) {
  return (
    <div className="flex flex-col gap-0.5 text-xs text-muted-foreground">
      {showTitle ? <p className="font-medium text-foreground">Audit details</p> : null}
      <p>
        {actorUsername || "Unknown actor"} / {action}
      </p>
    </div>
  )
}

export function AuditMetadataPreview({
  metadata,
  actorUsername,
  action,
}: {
  metadata: Record<string, unknown>
  actorUsername: string
  action: string
}) {
  const isMobile = useIsMobile()
  const formatted = formatMetadata(metadata)

  async function copyMetadata() {
    try {
      await copyText(formatted.full)
      toast.success("Metadata copied.")
    } catch {
      toast.error("Metadata copy failed.")
    }
  }

  if (isMobile) {
    return (
      <Drawer>
        <DrawerTrigger asChild>
          <button type="button" className="block w-full text-left">
            <MetadataPreviewCode preview={formatted.preview} />
          </button>
        </DrawerTrigger>
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>Audit details</DrawerTitle>
            <DrawerDescription asChild>
              <MetadataContext
                actorUsername={actorUsername}
                action={action}
                showTitle={false}
              />
            </DrawerDescription>
          </DrawerHeader>
          <div className="px-4 pb-4">
            <MetadataDetail
              full={formatted.full}
              onCopy={copyMetadata}
              showTitle
            />
          </div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <HoverCard>
      <HoverCardTrigger
        delay={hoverPreviewDelay}
        closeDelay={hoverPreviewCloseDelay}
        render={<span className="block w-full" />}
      >
        <MetadataPreviewCode preview={formatted.preview} />
      </HoverCardTrigger>
      <HoverCardContent className="w-96 max-w-[90vw]">
        <MetadataDetail full={formatted.full} onCopy={copyMetadata} />
      </HoverCardContent>
    </HoverCard>
  )
}
