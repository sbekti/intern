import assert from "node:assert/strict"
import test from "node:test"

import {
  failMetricsSnapshot,
  mergeMetricsSnapshot,
  metricsSnapshotSeriesKey,
  validateMetricsSnapshot,
  type MetricsDashboardSnapshot,
  type MetricsSnapshot,
  type MetricsSnapshotIdentity,
} from "./metrics-snapshot-client.ts"

const identities: readonly MetricsSnapshotIdentity[] = [
  { group: "group-a", lane: "lane-a", series: "series-a" },
  { group: "group-a", lane: "lane-a", series: "series-b" },
]
const request = { start: 100, end: 200, stepSeconds: 10 }

function validPayload(): MetricsSnapshot {
  return {
    version: 1,
    start: request.start,
    end: request.end,
    step_seconds: request.stepSeconds,
    series: [
      {
        ...identities[0],
        status: "ok",
        points: [
          [100, 1],
          [110, 2],
        ],
      },
      { ...identities[1], status: "error" },
    ],
  }
}

test("strictly validates the exact batch envelope and identities", () => {
  assert.deepEqual(
    validateMetricsSnapshot(validPayload(), request, identities),
    validPayload()
  )

  const wrongRange = { ...validPayload(), end: 190 }
  assert.equal(validateMetricsSnapshot(wrongRange, request, identities), null)

  const duplicate = validPayload()
  duplicate.series[1] = { ...duplicate.series[0] }
  assert.equal(validateMetricsSnapshot(duplicate, request, identities), null)

  const unknown = validPayload()
  unknown.series[1] = {
    group: "group-a",
    lane: "lane-a",
    series: "unknown-series",
    status: "error",
  }
  assert.equal(validateMetricsSnapshot(unknown, request, identities), null)

  const missing = validPayload()
  missing.series.pop()
  assert.equal(validateMetricsSnapshot(missing, request, identities), null)
})

test("rejects malformed status shapes and unordered or non-finite points", () => {
  const extraField = validPayload()
  ;(extraField.series[0] as unknown as Record<string, unknown>).extra = true
  assert.equal(validateMetricsSnapshot(extraField, request, identities), null)

  const unordered = validPayload()
  unordered.series[0] = {
    ...unordered.series[0],
    status: "ok",
    points: [
      [110, 2],
      [100, 1],
    ],
  }
  assert.equal(validateMetricsSnapshot(unordered, request, identities), null)

  const nonFinite = validPayload()
  nonFinite.series[0] = {
    ...nonFinite.series[0],
    status: "ok",
    points: [[100, Number.NaN]],
  }
  assert.equal(validateMetricsSnapshot(nonFinite, request, identities), null)
})

test("atomically merges successful points and retains partial failures as stale", () => {
  const previous: MetricsDashboardSnapshot = {
    range: { start: 100, end: 200 },
    stepSeconds: 10,
    series: {
      [metricsSnapshotSeriesKey(identities[0])]: {
        status: "live",
        points: [
          [100, 1],
          [110, 2],
        ],
      },
      [metricsSnapshotSeriesKey(identities[1])]: {
        status: "live",
        points: [[100, 3]],
      },
    },
  }
  const response: MetricsSnapshot = {
    version: 1,
    start: 160,
    end: 200,
    step_seconds: 10,
    series: [
      {
        ...identities[0],
        status: "ok",
        points: [
          [160, 16],
          [200, 20],
        ],
      },
      { ...identities[1], status: "error" },
    ],
  }

  const next = mergeMetricsSnapshot(previous, response, {
    start: 100,
    end: 200,
  })
  assert.deepEqual(next.range, { start: 100, end: 200 })
  assert.equal(next.stepSeconds, 10)
  assert.deepEqual(next.series[metricsSnapshotSeriesKey(identities[0])], {
    status: "live",
    points: [
      [100, 1],
      [110, 2],
      [160, 16],
      [200, 20],
    ],
  })
  assert.deepEqual(next.series[metricsSnapshotSeriesKey(identities[1])], {
    status: "stale",
    points: [[100, 3]],
  })
})

test("marks first-load request failures unavailable and retains later data as stale", () => {
  const range = { start: 100, end: 200 }
  const initial = failMetricsSnapshot(null, identities, range, 10)
  assert.ok(
    identities.every(
      (identity) =>
        initial.series[metricsSnapshotSeriesKey(identity)].status ===
        "unavailable"
    )
  )

  const previous: MetricsDashboardSnapshot = {
    range,
    stepSeconds: 10,
    series: {
      [metricsSnapshotSeriesKey(identities[0])]: {
        status: "live",
        points: [[180, 18]],
      },
      [metricsSnapshotSeriesKey(identities[1])]: {
        status: "unavailable",
        points: [],
      },
    },
  }
  const failed = failMetricsSnapshot(previous, identities, range, 10)
  assert.equal(
    failed.series[metricsSnapshotSeriesKey(identities[0])].status,
    "stale"
  )
  assert.deepEqual(
    failed.series[metricsSnapshotSeriesKey(identities[0])].points,
    [[180, 18]]
  )
  assert.equal(
    failed.series[metricsSnapshotSeriesKey(identities[1])].status,
    "unavailable"
  )
})
