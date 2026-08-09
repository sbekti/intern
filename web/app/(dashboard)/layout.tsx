import type { ReactNode } from "react"

import { AppSidebar } from "@/components/app-sidebar"
import { DashboardShellProvider } from "@/components/dashboard-shell-provider"
import { SiteHeader } from "@/components/site-header"
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { getProfile } from "@/lib/api"
import { getFrontendSsoConfig } from "@/lib/frontend-config"

export default async function DashboardLayout({
  children,
}: Readonly<{
  children: ReactNode
}>) {
  const profile = await getProfile()
  const frontendSso = getFrontendSsoConfig()

  if (!profile.ok) {
    throw new Error("Required authenticated profile unavailable for dashboard shell.")
  }

  return (
    <SidebarProvider
      style={
        {
          "--sidebar-width": "calc(var(--spacing) * 72)",
          "--header-height": "calc(var(--spacing) * 12)",
        } as React.CSSProperties
      }
    >
      <DashboardShellProvider isAdmin={profile.data.is_admin}>
        <AppSidebar
          variant="inset"
          user={{
            name: profile.data.name,
            email: profile.data.email,
            avatar: "",
          }}
          isAdmin={profile.data.is_admin}
          logoutUrl={frontendSso.logoutUrl}
        />
        <SidebarInset>
          <SiteHeader />
          <div className="flex flex-1 flex-col">
            <div className="@container/main flex flex-1 flex-col gap-4 py-4 md:py-6">
              {children}
            </div>
          </div>
        </SidebarInset>
      </DashboardShellProvider>
    </SidebarProvider>
  )
}
