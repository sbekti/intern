import assert from "node:assert/strict"
import test from "node:test"
import { withQuery } from "./query-string.ts"

test("query URLs omit empty values, retain zero offsets, and avoid empty suffixes", () => {
  assert.equal(
    withQuery("/profile/security", {
      tab: undefined,
      unused: null,
      empty: "",
      limit: 25,
      offset: 0,
    }),
    "/profile/security?limit=25&offset=0"
  )
  assert.equal(
    withQuery("/admin/audit-logs", { action: "" }),
    "/admin/audit-logs"
  )
})

test("query URLs encode literal search text without changing its value", () => {
  const text = "Camera & Office_LAN + 100% #1"
  const url = withQuery("/api/v1/admin/audit_logs", { metadata_contains: text })
  assert.equal(
    new URL(url, "https://example.test").searchParams.get("metadata_contains"),
    text
  )
})
