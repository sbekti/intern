import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { SecurityLoadingPanel } from "@/components/loading-panels"
import { SecuritySessions } from "@/components/security-sessions"
import { getProfile, listAdminAuthSessions, listProfileSessions } from "@/lib/api"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"
const defaultAdminPageSize = 25
const allowedAdminPageSizes = [25, 50, 100, 200] as const

type SearchParams = Promise<Record<string, string | string[] | undefined>>

export const metadata = createPageMetadata("/profile/security")

function readParam(
  params: Record<string, string | string[] | undefined>,
  key: string
) {
  const value = params[key]
  if (Array.isArray(value)) {
    return value[0] ?? ""
  }
  return value ?? ""
}

function parseOffset(value: string) {
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0
  }
  return parsed
}

function parsePageSize(value: string) {
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed)) {
    return defaultAdminPageSize
  }
  if (
    allowedAdminPageSizes.includes(
      parsed as (typeof allowedAdminPageSizes)[number]
    )
  ) {
    return parsed
  }
  return defaultAdminPageSize
}

export default async function ProfileSecurityPage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams
  const requestedTab = readParam(params, "tab") === "all" ? "all" : "mine"
  const limit = parsePageSize(readParam(params, "limit"))
  const offset = parseOffset(readParam(params, "offset"))
  const profile = await getProfile()

  if (!profile.ok) {
    return <UnauthorizedState />
  }

  if (hasForcedGlimmer(params)) {
    return <SecurityLoadingPanel isAdmin={profile.data.is_admin} />
  }

  const sessions = await listProfileSessions({ limit, offset })

  if (!sessions.ok) {
    return <UnauthorizedState />
  }

  const activeTab = profile.data.is_admin && requestedTab === "all" ? "all" : "mine"
  const adminSessions =
    profile.data.is_admin && activeTab === "all"
      ? await listAdminAuthSessions({ limit, offset })
      : null

  if (adminSessions && !adminSessions.ok) {
    if (adminSessions.status === 401) {
      return <UnauthorizedState />
    }

    return <ForbiddenState />
  }

  return (
    <div className="grid gap-4 px-4 lg:px-6">
      <SecuritySessions
        isAdmin={profile.data.is_admin}
        activeTab={activeTab}
        personalSessions={sessions.data}
        adminSessions={adminSessions?.data ?? null}
        adminPageSizes={allowedAdminPageSizes}
      />
    </div>
  )
}
