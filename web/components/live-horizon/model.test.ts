import assert from "node:assert/strict"
import test from "node:test"

import {
  clampedLabelPosition,
  mergeTimeSeriesPoints,
  overlappingTickIndexes,
  parseBoundedTimeRange,
  timeRangeFraction,
  timeRangeTimestampAtFraction,
  timeRangeTicks,
  timeSeriesExtent,
} from "./model.ts"

test("accepts a bounded time range", () => {
  const params = new URLSearchParams({ start: "100", end: "190" })
  assert.deepEqual(parseBoundedTimeRange(params, 30, 4), {
    start: 100,
    end: 190,
  })
})

test("calculates a focused position within a time range", () => {
  const range = { start: 100, end: 200 }

  assert.equal(timeRangeFraction(range, 100), 0)
  assert.equal(timeRangeFraction(range, 150), 0.5)
  assert.equal(timeRangeFraction(range, 200), 1)
  assert.equal(timeRangeFraction(range, 99), null)
  assert.equal(timeRangeFraction(range, null), null)
})

test("snaps a pointer fraction to the nearest time-series step", () => {
  const range = { start: 100, end: 200 }

  assert.equal(timeRangeTimestampAtFraction(range, -1, 10), 100)
  assert.equal(timeRangeTimestampAtFraction(range, 0.54, 10), 150)
  assert.equal(timeRangeTimestampAtFraction(range, 0.56, 10), 160)
  assert.equal(timeRangeTimestampAtFraction(range, 2, 10), 200)
  assert.equal(timeRangeTimestampAtFraction(range, 0.5, 0), null)
})

test("creates responsive ticks aligned to local clock boundaries", () => {
  const start = new Date(2026, 0, 15, 12, 2).getTime() / 1000
  const range = { start, end: start + 60 * 60 }
  const wideTicks = timeRangeTicks(range, 960)
  const narrowTicks = timeRangeTicks(range, 320)

  assert.equal(wideTicks.length, 12)
  assert.ok(
    wideTicks.every((timestamp) => {
      const date = new Date(timestamp * 1000)
      return date.getMinutes() % 5 === 0 && date.getSeconds() === 0
    })
  )
  assert.equal(narrowTicks.length, 4)
  assert.ok(
    narrowTicks.every((timestamp) => {
      const date = new Date(timestamp * 1000)
      return date.getMinutes() % 15 === 0 && date.getSeconds() === 0
    })
  )
})

test("finds every static tick obscured by the focus label", () => {
  assert.deepEqual(
    overlappingTickIndexes([0, 25, 50, 75, 100], 50, 20),
    [1, 2, 3]
  )
})

test("keeps timestamp labels within the axis", () => {
  assert.equal(clampedLabelPosition(5, 100, 20), 10)
  assert.equal(clampedLabelPosition(50, 100, 20), 50)
  assert.equal(clampedLabelPosition(95, 100, 20), 90)
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
