import type { AuditLogPage } from "./api"
import { filterOptions, type FilterField, type Sort } from "./list-controls.ts"

export const auditFilterLabels = {
  action: "Action",
  resource_type: "Resource Type",
  resource_id: "Resource ID",
  actor_username: "Actor",
}
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
  return Object.entries(auditFilterLabels).map(([key, label]) => ({
    key,
    label,
    allowCustom: true,
    options: filterOptions(
      items.map((item) => item[key as keyof typeof auditFilterLabels])
    ),
  }))
}
