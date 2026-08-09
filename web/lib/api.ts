import { headers } from "next/headers"

export type Profile = {
  username: string
  name: string
  email: string
  groups: string[]
  is_admin: boolean
}

export type Dashboard = {
  welcome_message: string
  profile: Profile
  network_summary: {
    device_count: number
    vlan_count: number
  }
  weather?: {
    location_name: string
    timezone: string
    current: {
      temperature_c: number
      wind_speed_kph: number
      weather_code: number
    }
  }
}

export type Vlan = {
  name: string
  vlan_id: number
  description: string
  created_at: string
  updated_at: string
}

export type NetworkDevice = {
  id: string
  mac_address: string
  display_name: string
  disabled: boolean
  vlan: {
    name: string
    vlan_id: number
  }
  created_at: string
  updated_at: string
}

export type AuthSession = {
  id: string
  username: string
  client_name: string
  is_current: boolean
  created_at: string
  last_used_at?: string
  idle_expires_at: string
  expires_at: string
}

export type AuditLogEntry = {
  id: string
  actor_username: string
  action: string
  resource_type: string
  resource_id: string
  metadata: Record<string, unknown>
  created_at: string
}

export type AuditLogPage = {
  items: AuditLogEntry[]
  pagination: {
    limit: number
    offset: number
    total: number
  }
}

export type AuthSessionPage = {
  items: AuthSession[]
  pagination: {
    limit: number
    offset: number
    total: number
  }
}

export type ApiResult<T> =
  | { ok: true; data: T }
  | { ok: false; status: 401 | 403 | 404 }

type AllowedStatus = 401 | 403 | 404

function isAllowedStatus(status: number): status is AllowedStatus {
  return status === 401 || status === 403 || status === 404
}

export async function resolveApiBaseUrl() {
  if (process.env.INTERN_API_BASE_URL) {
    return process.env.INTERN_API_BASE_URL.replace(/\/$/, "")
  }

  const requestHeaders = await headers()
  const host =
    requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host")

  if (!host) {
    throw new Error("Unable to resolve API host. Set INTERN_API_BASE_URL.")
  }

  const proto =
    requestHeaders.get("x-forwarded-proto") ??
    (host.startsWith("localhost") || host.startsWith("127.0.0.1")
      ? "http"
      : "https")

  return `${proto}://${host}`
}

export async function buildForwardHeaders() {
  const requestHeaders = await headers()
  const outbound = new Headers()

  const passthrough = [
    "authorization",
    "cookie",
    "x-intern-forward-auth",
    "remote-user",
    "remote-name",
    "remote-email",
    "remote-groups",
  ] as const

  for (const name of passthrough) {
    const value = requestHeaders.get(name)

    if (value) {
      outbound.set(name, value)
    }
  }

  return outbound
}

async function getJson<T>(path: string): Promise<ApiResult<T>> {
  const [baseUrl, forwardedHeaders] = await Promise.all([
    resolveApiBaseUrl(),
    buildForwardHeaders(),
  ])

  let response: Response

  try {
    response = await fetch(`${baseUrl}${path}`, {
      headers: forwardedHeaders,
      cache: "no-store",
    })
  } catch (error) {
    throw new Error(
      `Required API request failed for ${path}: ${
        error instanceof Error ? error.message : "unknown error"
      }`
    )
  }

  if (response.ok) {
    return {
      ok: true,
      data: (await response.json()) as T,
    }
  }

  if (isAllowedStatus(response.status)) {
    return {
      ok: false,
      status: response.status,
    }
  }

  throw new Error(
    `Required API request failed for ${path} with status ${response.status}`
  )
}

export function getProfile() {
  return getJson<Profile>("/api/v1/profile")
}

export function getDashboard() {
  return getJson<Dashboard>("/api/v1/dashboard")
}

export function listVlans() {
  return getJson<{ items: Vlan[] }>("/api/v1/networks/vlans")
}

export function listDevices() {
  return getJson<{ items: NetworkDevice[] }>("/api/v1/networks/devices")
}

export function listProfileSessions(filters: {
  limit?: number
  offset?: number
}) {
  const query = new URLSearchParams()

  for (const [key, rawValue] of Object.entries(filters)) {
    if (rawValue === undefined || rawValue === null) {
      continue
    }

    query.set(key, String(rawValue))
  }

  const suffix = query.size > 0 ? `?${query.toString()}` : ""
  return getJson<AuthSessionPage>(`/api/v1/profile/sessions${suffix}`)
}

export function listAdminAuthSessions(filters: {
  limit?: number
  offset?: number
}) {
  const query = new URLSearchParams()

  for (const [key, rawValue] of Object.entries(filters)) {
    if (rawValue === undefined || rawValue === null) {
      continue
    }

    query.set(key, String(rawValue))
  }

  const suffix = query.size > 0 ? `?${query.toString()}` : ""
  return getJson<AuthSessionPage>(`/api/v1/admin/auth/sessions${suffix}`)
}

export function listAdminAuditLogs(filters: {
  action?: string
  resource_type?: string
  resource_id?: string
  actor_username?: string
  limit?: number
  offset?: number
}) {
  const query = new URLSearchParams()

  for (const [key, rawValue] of Object.entries(filters)) {
    if (rawValue === undefined || rawValue === null || rawValue === "") {
      continue
    }

    query.set(key, String(rawValue))
  }

  const suffix = query.size > 0 ? `?${query.toString()}` : ""
  return getJson<AuditLogPage>(`/api/v1/admin/audit_logs${suffix}`)
}
