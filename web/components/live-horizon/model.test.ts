import assert from "node:assert/strict"
import test from "node:test"

import {
  isTimeSeriesPayload,
  mergeTimeSeriesPoints,
  parseBoundedTimeRange,
  timeSeriesExtent,
} from "./model.ts"

test("accepts a bounded time range", () => {
  const params = new URLSearchParams({ start: "100", end: "190" })
  assert.deepEqual(parseBoundedTimeRange(params, 30, 4), {
    start: 100,
    end: 190,
  })
})

test("rejects malformed, reversed, and oversized ranges", () => {
  assert.equal(
    parseBoundedTimeRange(
      new URLSearchParams({ start: "1.5", end: "2" }),
      30,
      10
    ),
    null
  )
  assert.equal(
    parseBoundedTimeRange(
      new URLSearchParams({ start: "31", end: "30" }),
      30,
      10
    ),
    null
  )
  assert.equal(
    parseBoundedTimeRange(
      new URLSearchParams({ start: "0", end: "120" }),
      30,
      4
    ),
    null
  )
})

test("merges overlap by timestamp and drops expired points", () => {
  assert.deepEqual(
    mergeTimeSeriesPoints(
      [
        [70, 1],
        [100, 2],
        [130, 3],
      ],
      [
        [100, 20],
        [130, 30],
        [160, 40],
      ],
      100
    ),
    [
      [100, 20],
      [130, 30],
      [160, 40],
    ]
  )
})

test("validates a generic time-series payload", () => {
  assert.equal(
    isTimeSeriesPayload(
      {
        step_seconds: 30,
        start: 100,
        end: 130,
        points: [
          [100, 12.5],
          [130, 18],
        ],
      },
      30
    ),
    true
  )
  assert.equal(
    isTimeSeriesPayload(
      {
        step_seconds: 60,
        start: 100,
        end: 130,
        points: [[100, 12.5]],
      },
      30
    ),
    false
  )
})

test("derives a Cubism-style extent from visible points", () => {
  assert.deepEqual(
    timeSeriesExtent([
      [100, 14],
      [130, 17],
    ]),
    [0, 17]
  )
  assert.deepEqual(
    timeSeriesExtent([
      [100, -4],
      [130, 2],
    ]),
    [-4, 4]
  )
  assert.deepEqual(timeSeriesExtent([]), [-1, 1])
})

test("adds optional headroom to an automatic extent", () => {
  assert.deepEqual(
    timeSeriesExtent(
      [
        [100, 10],
        [130, 20],
      ],
      0.5
    ),
    [0, 30]
  )
})
