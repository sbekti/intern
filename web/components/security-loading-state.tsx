"use client"

import { SecurityLoadingPanel } from "@/components/loading-panels"
import { useDashboardShell } from "@/components/dashboard-shell-provider"

export function SecurityLoadingState() {
  const { isAdmin } = useDashboardShell()

  return <SecurityLoadingPanel isAdmin={isAdmin} />
}
