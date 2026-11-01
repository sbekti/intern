import assert from "node:assert/strict"
import test from "node:test"
import {
  defaultDashboardLayout,
  moveSection,
  orderedSections,
  readDashboardLayout,
} from "./dashboard-layout.ts"
const groups = [{ id: "a" }, { id: "b" }, { id: "c" }]
test("defaults follow configuration; custom order appends new sections and ignores removed IDs", () => {
  assert.deepEqual(orderedSections(groups, defaultDashboardLayout), groups)
  assert.deepEqual(
    orderedSections(groups, {
      section_order: ["b", "missing", "b"],
      hidden_sections: [],
    }).map((g) => g.id),
    ["b", "a", "c"]
  )
})
test("invalid preference data is safe and hidden sections retain their order", () => {
  assert.deepEqual(readDashboardLayout(null), defaultDashboardLayout)
  const layout = readDashboardLayout({
    section_order: ["b", "b", 3, "a"],
    hidden_sections: ["b", "b"],
  })
  assert.deepEqual(layout, {
    section_order: ["b", "a"],
    hidden_sections: ["b"],
  })
  assert.deepEqual(
    orderedSections(groups, layout)
      .filter((g) => !layout.hidden_sections.includes(g.id))
      .map((g) => g.id),
    ["a", "c"]
  )
  assert.deepEqual(orderedSections(groups, defaultDashboardLayout), groups)
})
test("drag and menu moves produce the same stable order", () => {
  assert.deepEqual(moveSection(groups, 0, 2), [groups[1], groups[2], groups[0]])
  assert.deepEqual(moveSection(groups, 0, -1), groups)
})
