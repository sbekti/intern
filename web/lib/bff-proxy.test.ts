import assert from "node:assert/strict"
import test from "node:test"

import { resolveBffTarget } from "./bff-proxy.ts"

const baseUrl = "http://backend.example.test:8080"

test("browser API families retain their paths and queries", () => {
  for (const path of [
    ["admin", "auth", "sessions"],
    ["auth", "device_codes", "ABCD-EFGH", "approve"],
    ["gizmos", "device-1"],
    ["networks", "devices", "device-1"],
    ["profile", "sessions", "session-1", "revoke"],
  ]) {
    const target = resolveBffTarget(baseUrl, path, "?limit=25&offset=0")
    assert.equal(target?.origin, baseUrl)
    assert.equal(target?.pathname, `/api/v1/${path.join("/")}`)
    assert.equal(target?.search, "?limit=25&offset=0")
  }
})

test("unlisted API families are not exposed by the browser proxy", () => {
  for (const path of [
    [],
    ["kiosks", "02:00:00:00:00:01"],
    ["radius", "mab-snapshot"],
    ["system", "ping"],
    ["future-internal-service", "status"],
  ]) {
    assert.equal(resolveBffTarget(baseUrl, path, ""), null)
  }
})

test("normalization cannot leave the API base or enter an unlisted family", () => {
  for (const path of [
    ["..", "..", "readyz"],
    ["../../readyz"],
    ["profile", "..", "kiosks", "02:00:00:00:00:01"],
    ["profile/../kiosks/02:00:00:00:00:01"],
    ["%2e%2e", "%2e%2e", "readyz"],
    ["profile%2f..%2fkiosks", "02:00:00:00:00:01"],
    ["https://other.example.test"],
    ["", "", "other.example.test"],
    ["", "", "other.example.test", "api", "v1", "profile"],
  ]) {
    assert.equal(resolveBffTarget(baseUrl, path, ""), null)
  }
})

test("reserved characters stay inside their path segment", () => {
  const target = resolveBffTarget(
    baseUrl,
    ["gizmos", "id/with?reserved#characters"],
    "?limit=25"
  )
  assert.equal(
    target?.pathname,
    "/api/v1/gizmos/id%2Fwith%3Freserved%23characters"
  )
  assert.equal(target?.search, "?limit=25")
})

test("a configured API base path stays intact", () => {
  const target = resolveBffTarget(`${baseUrl}/service`, ["profile"], "")
  assert.equal(target?.href, `${baseUrl}/service/api/v1/profile`)
})
