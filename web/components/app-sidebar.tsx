"use client"

import Link from "next/link"

import { NavMain } from "@/components/nav-main"
import { NavUser } from "@/components/nav-user"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import {
  HouseIcon,
  MonitorSmartphoneIcon,
  NetworkIcon,
  ScrollTextIcon,
  ServerIcon,
} from "lucide-react"

type SidebarUser = {
  name: string
  email: string
  avatar: string
}

export function AppSidebar({
  user,
  isAdmin,
  logoutUrl,
  ...props
}: React.ComponentProps<typeof Sidebar> & {
  user: SidebarUser
  isAdmin: boolean
  logoutUrl: string | null
}) {
  const homeItems = [
    {
      title: "Home",
      url: "/",
      icon: <HouseIcon />,
    },
  ]
  const networkItems = isAdmin
    ? [
        {
          title: "Devices",
          url: "/networks/devices",
          icon: <MonitorSmartphoneIcon />,
        },
        {
          title: "VLANs",
          url: "/networks/vlans",
          icon: <NetworkIcon />,
        },
      ]
    : []
  const administrationItems = isAdmin
    ? [
        {
          title: "Audit Logs",
          url: "/admin/audit-logs",
          icon: <ScrollTextIcon />,
        },
      ]
    : []

  return (
    <Sidebar collapsible="offcanvas" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              closeOnMobile
              className="data-[slot=sidebar-menu-button]:p-1.5!"
              render={<Link href="/" />}
            >
              <ServerIcon className="size-5!" />
              <span className="text-base font-semibold">Bektinet</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={homeItems} />
        {networkItems.length > 0 ? (
          <NavMain label="Network" items={networkItems} />
        ) : null}
        {administrationItems.length > 0 ? (
          <NavMain label="Administration" items={administrationItems} />
        ) : null}
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={user} logoutUrl={logoutUrl} />
      </SidebarFooter>
    </Sidebar>
  )
}
