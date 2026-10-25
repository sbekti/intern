import type { ReactNode } from "react"

export const responsiveCompactButtonClass = "sm:w-auto sm:gap-1 sm:px-2.5"

export function CompactButtonLabel({ children }: { children: ReactNode }) {
  return <span className="sr-only sm:not-sr-only">{children}</span>
}
