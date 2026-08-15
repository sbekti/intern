import assert from "node:assert/strict"
import test from "node:test"

import {
  defaultMetricTimePreset,
  metricTimePreset,
  metricTimePresets,
  metricTimeRangePath,
  metricTimeSteps,
} from "./metric-time-range.ts"

test("defines the supported lookback windows", () => {
  assert.deepEqual(
    metricTimePresets.map(({ id }) => id),
    ["1h", "6h", "24h", "3d", "7d"]
  )
  assert.deepEqual(metricTimeSteps, [10, 60, 300])
  assert.deepEqual(metricTimePreset("3d"), {
    id: "3d",
    durationSeconds: 3 * 24 * 60 * 60,
    stepSeconds: 300,
  })
  assert.equal(
    metricTimePreset("3d").durationSeconds /
      metricTimePreset("3d").stepSeconds +
      1,
    865
  )
})

test("falls back to the default lookback for missing or invalid values", () => {
  assert.equal(metricTimePreset(undefined), defaultMetricTimePreset)
  assert.equal(metricTimePreset("invalid"), defaultMetricTimePreset)
  assert.equal(metricTimePreset(["1h", "7d"]), defaultMetricTimePreset)
})

test("updates the lookback while preserving the rest of the URL", () => {
  assert.equal(
    metricTimeRangePath("https://example.test/?view=all#metrics", "3d"),
    "/?view=all&range=3d#metrics"
  )
  assert.equal(
    metricTimeRangePath(
      "https://example.test/?view=all&range=3d#metrics",
      "6h"
    ),
    "/?view=all#metrics"
  )
})
