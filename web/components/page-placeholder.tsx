import { HammerIcon } from "lucide-react"

import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"

export function PagePlaceholder({
  title,
  description,
}: {
  title: string
  description: string
}) {
  return (
    <div className="px-4 lg:px-6">
      <Empty className="min-h-[24rem] border bg-card shadow-xs">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <HammerIcon />
          </EmptyMedia>
          <EmptyTitle>{title}</EmptyTitle>
          <EmptyDescription>{description}</EmptyDescription>
        </EmptyHeader>
        <EmptyContent className="text-muted-foreground">
          UI wiring for this route is intentionally staged after the shared
          shell.
        </EmptyContent>
      </Empty>
    </div>
  )
}
