import type { AuditLogPage } from "./api"
import { filterOptions, type FilterField, type Sort } from "./list-controls.ts"

const auditFilters = [
  { key: "action", label: "Action" },
  { key: "resource_type", label: "Resource Type" },
  { key: "resource_id", label: "Resource ID" },
  { key: "actor_username", label: "Actor" },
] as const
export const auditFilterKeys = auditFilters.map(({ key }) => key)
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

export function auditFilterFields(items: AuditLogPage["items"]): FilterField[] {
  return auditFilters.map(({ key, label }) => ({
    key,
    label,
    allowCustom: true,
    options: filterOptions(items.map((item) => item[key])),
  }))
}
