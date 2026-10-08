import { ArrowUpRightIcon, ChevronRightIcon } from "lucide-react"

import { UnauthorizedState } from "@/components/api-state"
import { ButtonLink } from "@/components/button-link"
import { ProfileLoadingPanel } from "@/components/loading-panels"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Empty, EmptyDescription } from "@/components/ui/empty"
import { Separator } from "@/components/ui/separator"
import { getProfile } from "@/lib/api"
import { getFrontendSsoConfig } from "@/lib/frontend-config"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"

type SearchParams = Promise<Record<string, string | string[] | undefined>>

export const metadata = createPageMetadata("/profile")

export default async function ProfilePage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams

  if (hasForcedGlimmer(params)) {
    return <ProfileLoadingPanel />
  }

  const profile = await getProfile()
  const frontendSso = getFrontendSsoConfig()

  if (!profile.ok) {
    return <UnauthorizedState />
  }

  const { username, name, email, groups, is_admin } = profile.data
  const displayName = name.trim() || username
  const initials = displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((segment) => segment[0]?.toUpperCase() ?? "")
    .join("")

  return (
    <div className="px-4 lg:px-6">
      <div className="mx-auto grid w-full max-w-3xl gap-4">
        <Card>
          <CardHeader className="flex flex-row items-start gap-4">
            <Avatar size="lg" aria-hidden="true">
              <AvatarFallback>{initials || "IN"}</AvatarFallback>
            </Avatar>
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <div className="grid gap-1">
                <CardTitle>
                  <h2 className="wrap-anywhere">{displayName}</h2>
                </CardTitle>
                <CardDescription className="break-all">{email}</CardDescription>
              </div>
              <Badge variant="secondary">
                {is_admin ? "Administrator" : "Authenticated user"}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="@container">
            <dl className="grid gap-4">
              <div className="grid gap-2 @sm:grid-cols-[8rem_minmax(0,1fr)] @sm:gap-4">
                <dt className="font-medium">Username</dt>
                <dd className="min-w-0 break-all text-muted-foreground">
                  {username}
                </dd>
              </div>
              <div className="grid gap-2 @sm:grid-cols-[8rem_minmax(0,1fr)] @sm:gap-4">
                <dt className="font-medium">Groups</dt>
                <dd className="min-w-0">
                  {groups.length > 0 ? (
                    <div className="flex flex-wrap gap-2">
                      {groups.map((group) => (
                        <Badge
                          key={group}
                          variant="outline"
                          className="h-auto max-w-full"
                        >
                          <span className="min-w-0 break-all whitespace-normal">
                            {group}
                          </span>
                        </Badge>
                      ))}
                    </div>
                  ) : (
                    <Empty className="items-start p-0 text-left">
                      <EmptyDescription>No groups assigned.</EmptyDescription>
                    </Empty>
                  )}
                </dd>
              </div>
            </dl>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>
              <h2>Account</h2>
            </CardTitle>
            <CardDescription>Manage sign-in and sessions.</CardDescription>
          </CardHeader>
          <CardContent className="@container grid gap-4">
            <div className="flex flex-col gap-3 @lg:flex-row @lg:items-center @lg:justify-between @lg:gap-6">
              <div className="grid min-w-0 gap-1">
                <h3 className="font-medium">Security</h3>
                <p className="text-muted-foreground">
                  Manage your active sessions.
                </p>
              </div>
              <ButtonLink variant="outline" href="/profile/security">
                Manage sessions
                <ChevronRightIcon data-icon="inline-end" />
              </ButtonLink>
            </div>
            <Separator />
            <div className="flex flex-col gap-3 @lg:flex-row @lg:items-center @lg:justify-between @lg:gap-6">
              <div className="grid min-w-0 gap-1">
                <h3 className="font-medium">SSO settings</h3>
                <p className="text-muted-foreground">
                  {frontendSso.settingsUrl
                    ? "Change your password in SSO."
                    : "SSO settings are unavailable."}
                </p>
              </div>
              {frontendSso.settingsUrl ? (
                <ButtonLink
                  variant="outline"
                  href={frontendSso.settingsUrl}
                  target="_blank"
                  rel="noreferrer"
                >
                  Open SSO settings
                  <ArrowUpRightIcon data-icon="inline-end" />
                </ButtonLink>
              ) : (
                <Button variant="outline" disabled>
                  Open SSO settings
                  <ArrowUpRightIcon data-icon="inline-end" />
                </Button>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
