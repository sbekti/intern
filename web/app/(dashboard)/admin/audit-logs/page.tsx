import Link from "next/link"
import { ScrollTextIcon } from "lucide-react"

import { AuditLogPagination } from "@/components/audit-log-pagination"
import { AuditMetadataPreview } from "@/components/audit-metadata-preview"
import { ForbiddenState, UnauthorizedState } from "@/components/api-state"
import { LocalTimestamp } from "@/components/local-timestamp"
import { AuditLogsLoadingPanel } from "@/components/loading-panels"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { listAdminAuditLogs } from "@/lib/api"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"

type SearchParams = Promise<Record<string, string | string[] | undefined>>

const defaultPageSize = 25
const allowedPageSizes = [25, 50, 100, 200] as const

export const metadata = createPageMetadata("/admin/audit-logs")

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
    return defaultPageSize
  }
  if (allowedPageSizes.includes(parsed as (typeof allowedPageSizes)[number])) {
    return parsed
  }
  return defaultPageSize
}

function buildQuery(values: Record<string, string | number | undefined>) {
  const params = new URLSearchParams()

  for (const [key, rawValue] of Object.entries(values)) {
    if (rawValue === undefined || rawValue === "") {
      continue
    }
    params.set(key, String(rawValue))
  }

  const query = params.toString()
  return query ? `/admin/audit-logs?${query}` : "/admin/audit-logs"
}

export default async function AuditLogsPage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams
  const action = readParam(params, "action")
  const resourceType = readParam(params, "resource_type")
  const resourceId = readParam(params, "resource_id")
  const actorUsername = readParam(params, "actor_username")
  const limit = parsePageSize(readParam(params, "limit"))
  const offset = parseOffset(readParam(params, "offset"))

  if (hasForcedGlimmer(params)) {
    return <AuditLogsLoadingPanel />
  }

  const auditLogs = await listAdminAuditLogs({
    action,
    resource_type: resourceType,
    resource_id: resourceId,
    actor_username: actorUsername,
    limit,
    offset,
  })

  if (!auditLogs.ok) {
    if (auditLogs.status === 401) {
      return <UnauthorizedState />
    }

    return <ForbiddenState />
  }

  const page = auditLogs.data
  const pageStart = page.pagination.total === 0 ? 0 : offset + 1
  const pageEnd = Math.min(offset + page.items.length, page.pagination.total)

  return (
    <div className="grid gap-4 px-4 lg:px-6">
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>Filters</CardTitle>
          <CardDescription>
            Narrow the audit stream by exact action, resource, or actor.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form action="/admin/audit-logs" className="flex flex-col gap-6">
            <FieldGroup className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Field>
                <FieldLabel htmlFor="action">Action</FieldLabel>
                <Input id="action" name="action" defaultValue={action} />
              </Field>
              <Field>
                <FieldLabel htmlFor="resource_type">Resource Type</FieldLabel>
                <Input
                  id="resource_type"
                  name="resource_type"
                  defaultValue={resourceType}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="resource_id">Resource ID</FieldLabel>
                <Input
                  id="resource_id"
                  name="resource_id"
                  defaultValue={resourceId}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="actor_username">Actor Username</FieldLabel>
                <Input
                  id="actor_username"
                  name="actor_username"
                  defaultValue={actorUsername}
                />
              </Field>
            </FieldGroup>
            <input type="hidden" name="limit" value={String(limit)} />
            <div className="flex items-center gap-2">
              <Button type="submit" size="sm">
                Apply Filters
              </Button>
              <Button
                variant="outline"
                size="sm"
                nativeButton={false}
                render={<Link href={buildQuery({ limit: defaultPageSize })} />}
              >
                Clear
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>Audit Stream</CardTitle>
          <CardDescription>
            {page.items.length === 0
              ? "No audit logs matched the current filters."
              : `Showing ${pageStart}-${pageEnd} of ${page.pagination.total} entries.`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {page.items.length === 0 ? (
            <Empty className="min-h-[16rem] border bg-muted/20">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <ScrollTextIcon />
                </EmptyMedia>
                <EmptyTitle>No audit logs</EmptyTitle>
                <EmptyDescription>
                  Adjust the filters or wait for new audit events to appear.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Actor</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Resource</TableHead>
                  <TableHead>Metadata</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {page.items.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell>
                      <LocalTimestamp value={entry.created_at} />
                    </TableCell>
                    <TableCell>{entry.actor_username}</TableCell>
                    <TableCell>{entry.action}</TableCell>
                    <TableCell className="align-top">
                      <div className="flex flex-col">
                        <span>{entry.resource_type}</span>
                        <span className="max-w-[22rem] truncate text-xs text-muted-foreground">
                          {entry.resource_id}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="min-w-[20rem] max-w-[28rem] whitespace-normal">
                      <AuditMetadataPreview
                        metadata={entry.metadata}
                        actorUsername={entry.actor_username}
                        action={entry.action}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
        {page.items.length > 0 ? (
          <CardFooter>
            <AuditLogPagination
              action={action}
              resourceType={resourceType}
              resourceId={resourceId}
              actorUsername={actorUsername}
              limit={page.pagination.limit}
              offset={page.pagination.offset}
              total={page.pagination.total}
              pageSizes={allowedPageSizes}
            />
          </CardFooter>
        ) : null}
      </Card>
    </div>
  )
}
