"use client"

import { cn } from "@/lib/utils"
import {
  formatCompactLocalTimestamp,
  formatExactLocalTimestamp,
} from "@/lib/date-time"
import {
  hoverPreviewCloseDelay,
  hoverPreviewDelay,
} from "@/lib/hover"
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "@/components/ui/hover-card"

export function LocalTimestamp({
  value,
  emptyLabel = "Never",
  className,
}: {
  value?: string | null
  emptyLabel?: string
  className?: string
}) {
  if (!value) {
    return <span className={className}>{emptyLabel}</span>
  }

  const compactTimestamp = formatCompactLocalTimestamp(value)
  const exactTimestamp = formatExactLocalTimestamp(value)

  return (
    <HoverCard>
      <HoverCardTrigger
        delay={hoverPreviewDelay}
        closeDelay={hoverPreviewCloseDelay}
        render={
          <time
            dateTime={value}
            className={cn("inline-flex w-fit cursor-default", className)}
            suppressHydrationWarning
          />
        }
      >
        {compactTimestamp}
      </HoverCardTrigger>
      <HoverCardContent className="w-fit max-w-xs rounded-md px-3 py-1.5 text-xs shadow-md ring-1 ring-foreground/10">
        <div className="flex flex-col gap-1">
          <p className="font-semibold">Local time</p>
          <p className="text-xs opacity-80">{exactTimestamp}</p>
        </div>
      </HoverCardContent>
    </HoverCard>
  )
}
