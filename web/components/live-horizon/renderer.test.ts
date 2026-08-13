import assert from "node:assert/strict"
import test from "node:test"

import { incrementalRedraw } from "./renderer.ts"

test("shifts one column and redraws only the overlap", () => {
  assert.deepEqual(incrementalRedraw(100, 130, 30, 100, 6), {
    shiftColumns: 1,
    redrawFrom: 94,
  })
})

test("redraws every newly exposed column after a larger shift", () => {
  assert.deepEqual(incrementalRedraw(100, 400, 30, 100, 6), {
    shiftColumns: 10,
    redrawFrom: 90,
  })
})

test("requires a full redraw for a new, reversed, or expired window", () => {
  assert.equal(incrementalRedraw(null, 130, 30, 100, 6), null)
  assert.equal(incrementalRedraw(130, 100, 30, 100, 6), null)
  assert.equal(incrementalRedraw(100, 3100, 30, 100, 6), null)
})
