import type { ReactNode } from "react"

export const responsiveCompactButtonClass = "sm:w-auto sm:gap-1 sm:px-2.5"
export const iconOnlyButtonClass = "gap-0 px-0"

export function CompactButtonLabel({
  children,
}: {
  children: ReactNode
}) {
  return <span className="sr-only sm:not-sr-only">{children}</span>
}

export function IconOnlyButtonLabel({
  children,
}: {
  children: ReactNode
}) {
  return <span className="sr-only">{children}</span>
}
