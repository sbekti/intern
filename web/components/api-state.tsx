import { LockIcon, ShieldAlertIcon } from "lucide-react"

import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"

export function UnauthorizedState({
}: Record<string, never>) {
  return (
    <div className="px-4 lg:px-6">
      <Empty className="min-h-[20rem] border bg-card shadow-xs">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <LockIcon />
          </EmptyMedia>
          <EmptyTitle>Sign-in required</EmptyTitle>
          <EmptyDescription>
            Authentication is required before this view can load data from the
            backend.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    </div>
  )
}

export function ForbiddenState({
}: Record<string, never>) {
  return (
    <div className="px-4 lg:px-6">
      <Empty className="min-h-[20rem] border bg-card shadow-xs">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <ShieldAlertIcon />
          </EmptyMedia>
          <EmptyTitle>Access denied</EmptyTitle>
          <EmptyDescription>
            Your account is authenticated, but this screen requires
            administrative access.
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    </div>
  )
}
