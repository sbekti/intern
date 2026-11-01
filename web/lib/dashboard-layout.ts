export type DashboardLayout = {
  section_order: string[]
  hidden_sections: string[]
}
export const defaultDashboardLayout: DashboardLayout = {
  section_order: [],
  hidden_sections: [],
}

export function readDashboardLayout(value: unknown): DashboardLayout {
  const data =
    typeof value === "object" && value !== null
      ? (value as Record<string, unknown>)
      : {}
  const ids = (value: unknown) =>
    Array.isArray(value)
      ? [...new Set(value.filter((id): id is string => typeof id === "string"))]
      : []
  return {
    section_order: ids(data.section_order),
    hidden_sections: ids(data.hidden_sections),
  }
}

export function orderedSections<T extends { id: string }>(
  sections: readonly T[],
  layout: DashboardLayout
): T[] {
  const remaining = new Map(sections.map((section) => [section.id, section]))
  const ordered: T[] = []
  for (const id of layout.section_order) {
    const section = remaining.get(id)
    if (section) {
      ordered.push(section)
      remaining.delete(id)
    }
  }
  return [...ordered, ...remaining.values()]
}

export function moveSection<T>(
  items: readonly T[],
  from: number,
  to: number
): T[] {
  const next = [...items]
  if (from < 0 || to < 0 || from >= next.length || to >= next.length)
    return next
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}
