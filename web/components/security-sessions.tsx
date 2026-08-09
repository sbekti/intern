"use client"

import { useRouter } from "next/navigation"
import { useState, useTransition } from "react"
import { LaptopIcon, LoaderCircleIcon, LogOutIcon } from "lucide-react"
import { toast } from "sonner"

import type { AuthSession, AuthSessionPage } from "@/lib/api"
import { buildBffPath } from "@/lib/bff"
import { AuthSessionPagination } from "@/components/auth-session-pagination"
import {
  CompactButtonLabel,
  IconOnlyButtonLabel,
  responsiveCompactButtonClass,
  iconOnlyButtonClass,
} from "@/components/compact-button-label"
import { LocalTimestamp } from "@/components/local-timestamp"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

type SecuritySessionsProps = {
  isAdmin: boolean
  activeTab: "mine" | "all"
  personalSessions: AuthSessionPage
  adminSessions: AuthSessionPage | null
  adminPageSizes: readonly number[]
}

type SessionCardProps = {
  title: string
  description: string
  sessions: AuthSession[]
  showUsername: boolean
  pendingPath: string | null
  isPending: boolean
  revokePathFor: (session: AuthSession) => string
  onRevoke: (path: string, successMessage: string) => void
  action: React.ReactNode
  scopeControl?: React.ReactNode
  emptyTitle: string
  emptyDescription: string
  footer?: React.ReactNode
}

const personalBulkPath = buildBffPath("/profile/sessions/revoke_others")
const adminBulkPath = buildBffPath("/admin/auth/sessions/revoke_all")

function buildSecurityQuery(tab: "mine" | "all", limit?: number, offset?: number) {
  const params = new URLSearchParams()

  if (tab === "all") {
    params.set("tab", "all")
  }
  if (limit !== undefined) {
    params.set("limit", String(limit))
  }
  if (offset !== undefined) {
    params.set("offset", String(offset))
  }

  const query = params.toString()
  return query ? `/profile/security?${query}` : "/profile/security"
}

async function post(path: string) {
  const response = await fetch(path, {
    method: "POST",
  })

  if (!response.ok) {
    throw new Error(`Request failed with status ${response.status}`)
  }
}

function SessionCard({
  title,
  description,
  sessions,
  showUsername,
  pendingPath,
  isPending,
  revokePathFor,
  onRevoke,
  action,
  scopeControl,
  emptyTitle,
  emptyDescription,
  footer,
}: SessionCardProps) {
  return (
    <Card className="border-border/70 shadow-xs">
      <CardHeader>
        <div className="flex flex-col gap-1">
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <div className="min-w-0 flex-1">{scopeControl}</div>
          <div className="shrink-0">{action}</div>
        </div>
        {sessions.length === 0 ? (
          <Empty className="min-h-[16rem] border bg-muted/20">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <LaptopIcon />
              </EmptyMedia>
              <EmptyTitle>{emptyTitle}</EmptyTitle>
              <EmptyDescription>{emptyDescription}</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                {showUsername ? <TableHead>Username</TableHead> : null}
                <TableHead>Client</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead>Idle expiry</TableHead>
                <TableHead>Absolute expiry</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sessions.map((session) => {
                const path = revokePathFor(session)
                const busy = pendingPath === path

                return (
                  <TableRow key={session.id}>
                    {showUsername ? <TableCell>{session.username}</TableCell> : null}
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-foreground">
                          {session.client_name}
                        </span>
                        {session.is_current ? <Badge variant="secondary">Current</Badge> : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <LocalTimestamp value={session.last_used_at} />
                    </TableCell>
                    <TableCell>
                      <LocalTimestamp value={session.idle_expires_at} />
                    </TableCell>
                    <TableCell>
                      <LocalTimestamp value={session.expires_at} />
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="icon-sm"
                          className={iconOnlyButtonClass}
                          disabled={isPending}
                          onClick={() => onRevoke(path, "Client session signed out.")}
                          aria-label="Sign out session"
                        >
                          {busy ? (
                            <LoaderCircleIcon className="animate-spin" data-icon="inline-start" />
                          ) : (
                            <LogOutIcon data-icon="inline-start" />
                          )}
                          <IconOnlyButtonLabel>Sign out</IconOnlyButtonLabel>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
      {footer && sessions.length > 0 ? <CardFooter>{footer}</CardFooter> : null}
    </Card>
  )
}

export function SecuritySessions({
  isAdmin,
  activeTab,
  personalSessions,
  adminSessions,
  adminPageSizes,
}: SecuritySessionsProps) {
  const router = useRouter()
  const [pendingPath, setPendingPath] = useState<string | null>(null)
  const [dialogTarget, setDialogTarget] = useState<"mine" | "all" | null>(null)
  const [isPending, startTransition] = useTransition()

  const adminPage = adminSessions ?? {
    items: [],
    pagination: {
      limit: adminPageSizes[0] ?? 25,
      offset: 0,
      total: 0,
    },
  }

  function navigateToTab(nextTab: "mine" | "all") {
    if (nextTab === "all") {
      router.push(buildSecurityQuery("all", adminPage.pagination.limit, 0))
      return
    }

    router.push(buildSecurityQuery("mine", personalSessions.pagination.limit, 0))
  }

  function runAction(path: string, successMessage: string) {
    setPendingPath(path)
    startTransition(async () => {
      try {
        await post(path)
        toast.success(successMessage)
        router.refresh()
      } catch {
        toast.error("Session update failed. Try again.")
      } finally {
        setDialogTarget(null)
        setPendingPath(null)
      }
    })
  }

  const sessionScopeControl = isAdmin ? (
    <>
      <div className="lg:hidden">
        <Select
          value={activeTab}
          onValueChange={(value) => navigateToTab(value === "all" ? "all" : "mine")}
          items={[
            { label: "My sessions", value: "mine" },
            { label: "All users", value: "all" },
          ]}
        >
          <SelectTrigger size="sm" className="w-full min-w-0">
            <SelectValue />
          </SelectTrigger>
          <SelectContent align="start">
            <SelectGroup>
              <SelectItem value="mine">My sessions</SelectItem>
              <SelectItem value="all">All users</SelectItem>
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
      <ToggleGroup
        value={[activeTab]}
        onValueChange={(value) => {
          const nextValue = value[0]

          if (!nextValue) {
            return
          }
          navigateToTab(nextValue === "all" ? "all" : "mine")
        }}
        variant="outline"
        size="sm"
        className="hidden lg:flex"
      >
        <ToggleGroupItem value="mine" aria-label="Show my sessions">
          My sessions
        </ToggleGroupItem>
        <ToggleGroupItem value="all" aria-label="Show all users sessions">
          All users
        </ToggleGroupItem>
      </ToggleGroup>
    </>
  ) : null

  const personalCard = (
    <SessionCard
      title="Client Sessions"
      description={
        "Active client sessions. Your browser SSO session is not listed here."
      }
      sessions={personalSessions.items}
      showUsername={false}
      pendingPath={pendingPath}
      isPending={isPending}
      revokePathFor={(session) => buildBffPath(`/profile/sessions/${session.id}/revoke`)}
      onRevoke={runAction}
      scopeControl={activeTab === "mine" ? sessionScopeControl : null}
      action={
        <Button
          variant="outline"
          size="icon-sm"
          className={responsiveCompactButtonClass}
          disabled={isPending || personalSessions.pagination.total === 0}
          onClick={() => setDialogTarget("mine")}
          aria-label="Sign out all client sessions"
        >
          <LogOutIcon data-icon="inline-start" />
          <CompactButtonLabel>Sign out all clients</CompactButtonLabel>
        </Button>
      }
      emptyTitle="No active client sessions"
      emptyDescription="There are no active client sessions for this account."
      footer={
        <AuthSessionPagination
          tab="mine"
          limit={personalSessions.pagination.limit}
          offset={personalSessions.pagination.offset}
          total={personalSessions.pagination.total}
          pageSizes={adminPageSizes}
        />
      }
    />
  )

  const adminCard = (
    <SessionCard
      title="All Client Sessions"
      description={
        "Active client sessions across all users."
      }
      sessions={adminPage.items}
      showUsername
      pendingPath={pendingPath}
      isPending={isPending}
      revokePathFor={(session) => buildBffPath(`/admin/auth/sessions/${session.id}/revoke`)}
      onRevoke={runAction}
      scopeControl={activeTab === "all" ? sessionScopeControl : null}
      action={
        <Button
          variant="outline"
          size="icon-sm"
          className={responsiveCompactButtonClass}
          disabled={isPending || adminPage.pagination.total === 0}
          onClick={() => setDialogTarget("all")}
          aria-label="Sign out all users sessions"
        >
          <LogOutIcon data-icon="inline-start" />
          <CompactButtonLabel>Sign out everyone</CompactButtonLabel>
        </Button>
      }
      emptyTitle="No active client sessions"
      emptyDescription="There are no active client sessions across all users."
      footer={
        <AuthSessionPagination
          tab="all"
          limit={adminPage.pagination.limit}
          offset={adminPage.pagination.offset}
          total={adminPage.pagination.total}
          pageSizes={adminPageSizes}
        />
      }
    />
  )

  if (!isAdmin) {
    return (
      <>
        {personalCard}

        <AlertDialog open={dialogTarget === "mine"} onOpenChange={(open) => setDialogTarget(open ? "mine" : null)}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Sign out all clients?</AlertDialogTitle>
              <AlertDialogDescription>
                This will sign out all active client sessions for your account. Your
                current browser SSO session will stay active.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={pendingPath === personalBulkPath}>Cancel</AlertDialogCancel>
              <AlertDialogAction
                disabled={pendingPath === personalBulkPath}
                onClick={() => runAction(personalBulkPath, "All client sessions signed out.")}
              >
                {pendingPath === personalBulkPath ? (
                  <LoaderCircleIcon className="animate-spin" data-icon="inline-start" />
                ) : null}
                Sign out all
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </>
    )
  }

  return (
    <>
      {activeTab === "all" ? adminCard : personalCard}

      <AlertDialog open={dialogTarget === "mine"} onOpenChange={(open) => setDialogTarget(open ? "mine" : null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Sign out all clients?</AlertDialogTitle>
            <AlertDialogDescription>
              This will sign out all active client sessions for your account. Your
              current browser SSO session will stay active.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={pendingPath === personalBulkPath}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={pendingPath === personalBulkPath}
              onClick={() => runAction(personalBulkPath, "All client sessions signed out.")}
            >
              {pendingPath === personalBulkPath ? (
                <LoaderCircleIcon className="animate-spin" data-icon="inline-start" />
              ) : null}
              Sign out all
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={dialogTarget === "all"} onOpenChange={(open) => setDialogTarget(open ? "all" : null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Sign out everyone?</AlertDialogTitle>
            <AlertDialogDescription>
              This will sign out every active client session across all users,
              including your own API client sessions. Your current browser SSO
              session will stay active.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={pendingPath === adminBulkPath}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              disabled={pendingPath === adminBulkPath}
              onClick={() => runAction(adminBulkPath, "All client sessions signed out.")}
            >
              {pendingPath === adminBulkPath ? (
                <LoaderCircleIcon className="animate-spin" data-icon="inline-start" />
              ) : null}
              Sign out all
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
