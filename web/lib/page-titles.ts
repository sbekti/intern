import type { Metadata } from "next"

export const appName = "Bektinet"

export const pageTitles = {
  "/": "Home",
  "/profile": "Profile",
  "/profile/security": "Security",
  "/networks/devices": "Devices",
  "/networks/vlans": "VLANs",
  "/admin/audit-logs": "Audit Logs",
  "/auth/device": "Device Approval",
} as const

export function getPageTitle(pathname: string) {
  return pageTitles[pathname as keyof typeof pageTitles] ?? pageTitles["/"]
}

export function createPageMetadata(pathname: string): Metadata {
  return { title: getPageTitle(pathname) }
}
