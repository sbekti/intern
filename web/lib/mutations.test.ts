import assert from "node:assert/strict"
import { afterEach, mock, test } from "node:test"
import { mutate } from "./mutations.ts"

afterEach(() => mock.restoreAll())

test("sends the requested mutation once", async () => {
  const fetch = mock.method(
    globalThis,
    "fetch",
    async () => new Response(null, { status: 204 })
  )
  const options = { method: "DELETE" }
  await mutate("/bff/v1/networks/devices/device-1", options)
  assert.equal(fetch.mock.callCount(), 1)
  assert.deepEqual(fetch.mock.calls[0].arguments, [
    "/bff/v1/networks/devices/device-1",
    options,
  ])
})

test("reports a connection failure without retrying a write", async () => {
  const fetch = mock.method(globalThis, "fetch", async () => {
    throw new TypeError("Failed to fetch")
  })
  await assert.rejects(
    mutate("/save", { method: "POST" }),
    /Couldn't reach the server/
  )
  assert.equal(fetch.mock.callCount(), 1)
})

test("preserves useful API validation errors", async () => {
  mock.method(globalThis, "fetch", async () =>
    Response.json(
      { message: "MAC address already registered" },
      { status: 409 }
    )
  )
  await assert.rejects(
    mutate("/save", { method: "POST" }),
    /MAC address already registered/
  )
})

test("handles a proxy's non-JSON error response", async () => {
  mock.method(
    globalThis,
    "fetch",
    async () => new Response("Bad Gateway", { status: 502 })
  )
  await assert.rejects(
    mutate("/save", { method: "PATCH" }),
    /Request failed \(502\)/
  )
})
