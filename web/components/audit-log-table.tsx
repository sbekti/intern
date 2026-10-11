"use client"

import { ListTable, ListMobile } from "@/components/responsive-list"

import { ListSort } from "@/components/list-sort"
import {
  Item,
  ItemContent,
  ItemTitle,
  ItemDescription,
  ItemFooter,
  ItemGroup,
} from "@/components/ui/item"
import { ScrollTextIcon } from "lucide-react"
import { AuditLogPagination } from "@/components/audit-log-pagination"
import { AuditMetadataPreview } from "@/components/audit-metadata-preview"
import { LocalTimestamp } from "@/components/local-timestamp"
import { SortableTableHead } from "@/components/sortable-table-head"
import { TokenFilterBar } from "@/components/token-filter-bar"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useListControls } from "@/hooks/use-list-controls"
import type { AuditLogPage } from "@/lib/api"
import {
  auditFilterFields,
  auditFilterKeys,
  auditSortKeys,
  defaultAuditSort,
} from "@/lib/audit-log-list"

export function AuditLogTable({
  page,
  pageSizes,
}: {
  page: AuditLogPage
  pageSizes: readonly number[]
}) {
  const controls = useListControls(
    auditFilterKeys,
    auditSortKeys,
    defaultAuditSort,
    true
  )
  const start = page.pagination.total === 0 ? 0 : page.pagination.offset + 1
  const end = Math.min(
    page.pagination.offset + page.items.length,
    page.pagination.total
  )
  return (
    <Card className="border-border/70 shadow-xs" aria-busy={controls.pending}>
      <CardHeader>
        <CardTitle>Audit Stream</CardTitle>
        <CardDescription>
          {page.items.length === 0
            ? "No audit logs matched the current filters."
            : `Showing ${start}-${end} of ${page.pagination.total} entries.`}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <TokenFilterBar
          fields={auditFilterFields(page.items)}
          filters={controls.filters}
          onChange={controls.setFilters}
          disabled={controls.pending}
        />
        {page.items.length === 0 ? (
          <Empty className="min-h-[16rem] border bg-muted/20">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <ScrollTextIcon />
              </EmptyMedia>
              <EmptyTitle>No audit logs</EmptyTitle>
              <EmptyDescription>
                Change or clear the filters, or wait for new audit events to
                appear.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <>
            <ListTable>
              <Table>
                <TableHeader>
                  <TableRow>
                    <SortableTableHead
                      sortKey="created_at"
                      sort={controls.sort}
                      onSort={controls.setSort}
                      disabled={controls.pending}
                    >
                      Timestamp
                    </SortableTableHead>
                    <SortableTableHead
                      sortKey="actor_username"
                      sort={controls.sort}
                      onSort={controls.setSort}
                      disabled={controls.pending}
                    >
                      Actor
                    </SortableTableHead>
                    <SortableTableHead
                      sortKey="action"
                      sort={controls.sort}
                      onSort={controls.setSort}
                      disabled={controls.pending}
                    >
                      Action
                    </SortableTableHead>
                    <SortableTableHead
                      sortKey="resource"
                      sort={controls.sort}
                      onSort={controls.setSort}
                      disabled={controls.pending}
                    >
                      Resource
                    </SortableTableHead>
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
                      <TableCell className="max-w-[28rem] min-w-[20rem] whitespace-normal">
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
            </ListTable>
            <ListMobile>
              <ListSort
                sort={controls.sort}
                onSort={controls.setSort}
                disabled={controls.pending}
                options={[
                  { key: "created_at", label: "Timestamp" },
                  { key: "actor_username", label: "Actor" },
                  { key: "action", label: "Action" },
                  { key: "resource", label: "Resource" },
                ]}
              />
              <ItemGroup>
                {page.items.map((entry) => (
                  <Item
                    key={entry.id}
                    role="listitem"
                    variant="outline"
                    size="sm"
                  >
                    <ItemContent className="min-w-0">
                      <ItemTitle className="max-w-full min-w-0 wrap-anywhere">
                        {entry.action}
                      </ItemTitle>
                      <ItemDescription>{entry.actor_username}</ItemDescription>
                      <ItemDescription className="wrap-anywhere">
                        {entry.resource_type} · {entry.resource_id}
                      </ItemDescription>
                    </ItemContent>
                    <ItemFooter className="flex-wrap">
                      <LocalTimestamp value={entry.created_at} />
                      <AuditMetadataPreview
                        metadata={entry.metadata}
                        actorUsername={entry.actor_username}
                        action={entry.action}
                        compact
                      />
                    </ItemFooter>
                  </Item>
                ))}
              </ItemGroup>
            </ListMobile>
          </>
        )}
      </CardContent>
      {page.items.length ? (
        <CardFooter inert={controls.pending}>
          <AuditLogPagination
            filters={controls.filters}
            sort={controls.sort}
            limit={page.pagination.limit}
            offset={page.pagination.offset}
            total={page.pagination.total}
            pageSizes={pageSizes}
          />
        </CardFooter>
      ) : null}
    </Card>
  )
}
