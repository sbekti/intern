"use client"

import type { ReactNode } from "react"
import { useDashboardShell } from "@/components/dashboard-shell-provider"
import { cn } from "@/lib/utils"

export function ListTable({ children }: { children: ReactNode }) {
  const { listLayout } = useDashboardShell()
  return (
    <div
      data-slot="list-table"
      className={cn("hidden md:block", listLayout === "table" && "block")}
    >
      {children}
    </div>
  )
}

export function ListMobile({ children }: { children: ReactNode }) {
  const { listLayout } = useDashboardShell()
  return (
    <div
      data-slot="list-mobile"
      className={cn(
        "flex flex-col gap-3 md:hidden",
        listLayout === "table" && "hidden"
      )}
    >
      {children}
    </div>
  )
}
