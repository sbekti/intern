import { ArrowUpRightIcon } from "lucide-react"

import { UnauthorizedState } from "@/components/api-state"
import { ProfileLoadingPanel } from "@/components/loading-panels"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
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

  return (
    <div className="grid gap-4 px-4 lg:px-6">
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>{profile.data.name}</CardTitle>
          <CardDescription>{profile.data.email}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <div className="rounded-xl border bg-muted/30 p-4">
            <p className="text-sm font-medium text-foreground">Username</p>
            <p className="mt-2 text-sm text-muted-foreground">
              {profile.data.username}
            </p>
          </div>
          <div className="rounded-xl border bg-muted/30 p-4">
            <p className="text-sm font-medium text-foreground">Access</p>
            <p className="mt-2 text-sm text-muted-foreground">
              {profile.data.is_admin ? "Administrator" : "Authenticated user"}
            </p>
          </div>
        </CardContent>
      </Card>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>Groups</CardTitle>
          <CardDescription>
            Current group membership from the authenticated backend profile.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {profile.data.groups.map((group) => (
            <Badge key={group} variant="outline">
              {group}
            </Badge>
          ))}
        </CardContent>
      </Card>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>Password</CardTitle>
          <CardDescription>
            Password changes are handled by the central SSO settings page.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {frontendSso.settingsUrl ? (
            <Button
              nativeButton={false}
              render={
                <a
                  href={frontendSso.settingsUrl}
                  target="_blank"
                  rel="noreferrer"
                />
              }
            >
              <ArrowUpRightIcon data-icon="inline-end" />
              Open SSO Settings
            </Button>
          ) : (
            <Button disabled>
              <ArrowUpRightIcon data-icon="inline-end" />
              Open SSO Settings
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
