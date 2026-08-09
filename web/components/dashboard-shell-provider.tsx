"use client"

import { createContext, useContext, type ReactNode } from "react"

const DashboardShellContext = createContext<{ isAdmin: boolean }>({
  isAdmin: false,
})

export function DashboardShellProvider({
  isAdmin,
  children,
}: {
  isAdmin: boolean
  children: ReactNode
}) {
  return (
    <DashboardShellContext.Provider value={{ isAdmin }}>
      {children}
    </DashboardShellContext.Provider>
  )
}

export function useDashboardShell() {
  return useContext(DashboardShellContext)
}
