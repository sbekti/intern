"use client"

import { createContext, useContext, useState, type ReactNode } from "react"

import { listLayoutCookie, type ListLayout } from "@/lib/list-layout"

const DashboardShellContext = createContext<{
  isAdmin: boolean
  listLayout: ListLayout
  setListLayout: (layout: ListLayout) => void
}>({
  isAdmin: false,
  listLayout: "auto",
  setListLayout: () => {},
})

export function DashboardShellProvider({
  isAdmin,
  initialListLayout,
  children,
}: {
  isAdmin: boolean
  initialListLayout: ListLayout
  children: ReactNode
}) {
  const [listLayout, setLayout] = useState(initialListLayout)
  function setListLayout(layout: ListLayout) {
    setLayout(layout)
    try {
      document.cookie = `${listLayoutCookie}=${layout}; path=/; max-age=31536000; samesite=lax`
    } catch {
      // Keep the preference for this visit when cookies are blocked.
    }
  }

  return (
    <DashboardShellContext.Provider
      value={{ isAdmin, listLayout, setListLayout }}
    >
      {children}
    </DashboardShellContext.Provider>
  )
}

export function useDashboardShell() {
  return useContext(DashboardShellContext)
}
