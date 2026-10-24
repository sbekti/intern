import type { AuditLogPage } from "./api"
import {
  filterOptions,
  type FilterField,
  type Filters,
  type Sort,
} from "./list-controls.ts"

export const auditFilterKeys = [
  "actor_username_contains",
  "action",
  "resource_type",
  "resource_id",
  "metadata_contains",
  "actor_username",
] as const
export const auditSortKeys = [
  "created_at",
  "actor_username",
  "action",
  "resource",
] as const
export const defaultAuditSort: Sort<(typeof auditSortKeys)[number]> = {
  key: "created_at",
  direction: "desc",
}

export function auditFilterFields(
  options: AuditLogPage["filter_options"],
  filters: Filters
): FilterField[] {
  return [
    { key: "actor_username_contains", label: "Actor" },
    { key: "action", label: "Action", options: filterOptions(options.actions) },
    {
      key: "resource_type",
      label: "Resource Type",
      options: filterOptions(options.resource_types),
    },
    { key: "resource_id", label: "Resource ID", exact: true },
    { key: "metadata_contains", label: "Metadata" },
    ...(filters.actor_username
      ? [{ key: "actor_username", label: "Actor (exact)", exact: true }]
      : []),
  ]
}
