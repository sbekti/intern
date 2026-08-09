"use client"

import Link from "next/link"
import { startTransition, type MouseEvent, type ReactNode } from "react"
import { usePathname, useRouter } from "next/navigation"

import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

export function NavMain({
  label,
  items,
}: {
  label?: string
  items: {
    title: string
    url: string
    icon?: ReactNode
  }[]
}) {
  const pathname = usePathname()
  const router = useRouter()

  function isItemActive(url: string) {
    return url === "/"
      ? pathname === url
      : pathname === url || pathname.startsWith(`${url}/`)
  }

  function handleNavClick(event: MouseEvent<HTMLAnchorElement>, url: string) {
    if (!isItemActive(url)) {
      return
    }

    event.preventDefault()
    startTransition(() => {
      router.refresh()
    })
  }

  return (
    <SidebarGroup>
      {label ? <SidebarGroupLabel>{label}</SidebarGroupLabel> : null}
      <SidebarGroupContent>
        <SidebarMenu>
          {items.map((item) => (
            <SidebarMenuItem key={item.title}>
              <SidebarMenuButton
                closeOnMobile
                tooltip={item.title}
                isActive={isItemActive(item.url)}
                render={<Link href={item.url} onClick={(event) => handleNavClick(event, item.url)} />}
              >
                {item.icon}
                <span>{item.title}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ))}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}
