import type {
  TimeRange,
  TimeSeriesPoint,
} from "../components/live-horizon/model.ts"
import { mergeTimeSeriesPoints } from "../components/live-horizon/model.ts"

export type MetricsSnapshotIdentity = {
  group: string
  lane: string
  series: string
}

export type MetricsSnapshotSeries = MetricsSnapshotIdentity &
  (
    | {
        status: "ok"
        points: TimeSeriesPoint[]
      }
    | {
        status: "error"
      }
  )

export type MetricsSnapshot = {
  version: 1
  start: number
  end: number
  step_seconds: number
  series: MetricsSnapshotSeries[]
}

export type MetricsSeriesStatus = "loading" | "live" | "stale" | "unavailable"

export type MetricsSeriesSnapshot = {
  status: MetricsSeriesStatus
  points: readonly TimeSeriesPoint[]
}

export type MetricsDashboardSnapshot = {
  range: TimeRange
  stepSeconds: number
  series: Readonly<Record<string, MetricsSeriesSnapshot>>
}

export function metricsSnapshotSeriesKey({
  group,
  lane,
  series,
}: MetricsSnapshotIdentity) {
  return `${group}/${lane}/${series}`
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function exactKeys(
  value: Record<string, unknown>,
  expected: readonly string[]
) {
  const keys = Object.keys(value).sort()
  const expectedKeys = [...expected].sort()
  return (
    keys.length === expectedKeys.length &&
    keys.every((key, index) => key === expectedKeys[index])
  )
}

function validPoint(
  point: unknown,
  previousTimestamp: number,
  range: TimeRange
): point is TimeSeriesPoint {
  if (
    !Array.isArray(point) ||
    point.length !== 2 ||
    typeof point[0] !== "number" ||
    !Number.isFinite(point[0]) ||
    typeof point[1] !== "number" ||
    !Number.isFinite(point[1])
  ) {
    return false
  }

  return (
    point[0] > previousTimestamp &&
    point[0] >= range.start &&
    point[0] <= range.end
  )
}

function validSeries(
  value: unknown,
  expected: ReadonlySet<string>,
  seen: Set<string>,
  range: TimeRange
): value is MetricsSnapshotSeries {
  if (!isRecord(value)) {
    return false
  }

  const group = value.group
  const lane = value.lane
  const series = value.series
  if (
    typeof group !== "string" ||
    typeof lane !== "string" ||
    typeof series !== "string"
  ) {
    return false
  }

  const identity = { group, lane, series }
  const key = metricsSnapshotSeriesKey(identity)
  if (!expected.has(key) || seen.has(key)) {
    return false
  }
  seen.add(key)

  if (value.status === "error") {
    return exactKeys(value, ["group", "lane", "series", "status"])
  }

  if (
    value.status !== "ok" ||
    !Array.isArray(value.points) ||
    !exactKeys(value, ["group", "lane", "series", "status", "points"])
  ) {
    return false
  }

  let previousTimestamp = -Infinity
  for (const point of value.points) {
    if (!validPoint(point, previousTimestamp, range)) {
      return false
    }
    previousTimestamp = point[0]
  }

  return true
}

/**
 * Parses the browser-facing batch envelope against the exact request and
 * dashboard identities. A null result must be treated as a request failure.
 */
export function validateMetricsSnapshot(
  input: unknown,
  request: { start: number; end: number; stepSeconds: number },
  expectedIdentities: readonly MetricsSnapshotIdentity[]
): MetricsSnapshot | null {
  if (
    !isRecord(input) ||
    !exactKeys(input, ["version", "start", "end", "step_seconds", "series"]) ||
    input.version !== 1 ||
    input.start !== request.start ||
    input.end !== request.end ||
    input.step_seconds !== request.stepSeconds ||
    !Array.isArray(input.series)
  ) {
    return null
  }

  const expectedKeys = expectedIdentities.map(metricsSnapshotSeriesKey)
  const expected = new Set(expectedKeys)
  if (
    expected.size !== expectedKeys.length ||
    input.series.length !== expected.size
  ) {
    return null
  }

  const seen = new Set<string>()
  const range = { start: request.start, end: request.end }
  if (
    !input.series.every((series) => validSeries(series, expected, seen, range))
  ) {
    return null
  }
  if (seen.size !== expected.size) {
    return null
  }

  return input as MetricsSnapshot
}

function visiblePoints(
  points: readonly TimeSeriesPoint[] | undefined,
  range: TimeRange
) {
  return (
    points?.filter(
      ([timestamp]) => timestamp >= range.start && timestamp <= range.end
    ) ?? []
  )
}

function emptyOrRetained(
  previous: MetricsDashboardSnapshot | null,
  key: string,
  range: TimeRange
) {
  return visiblePoints(previous?.series[key]?.points, range)
}

/**
 * Applies a validated response as one immutable dashboard snapshot. Errors
 * retain their prior points, while successful series replace overlapping
 * timestamps and become live.
 */
export function mergeMetricsSnapshot(
  previous: MetricsDashboardSnapshot | null,
  response: MetricsSnapshot,
  range: TimeRange
): MetricsDashboardSnapshot {
  const series: Record<string, MetricsSeriesSnapshot> = {}

  for (const entry of response.series) {
    const key = metricsSnapshotSeriesKey(entry)
    const retained = emptyOrRetained(previous, key, range)
    if (entry.status === "ok") {
      series[key] = {
        status: "live",
        points: mergeTimeSeriesPoints(
          retained,
          entry.points,
          range.start
        ).filter(([timestamp]) => timestamp <= range.end),
      }
    } else {
      series[key] = {
        status: retained.length > 0 ? "stale" : "unavailable",
        points: retained,
      }
    }
  }

  return { range, stepSeconds: response.step_seconds, series }
}

/** Marks every configured series stale or unavailable without dropping data. */
export function failMetricsSnapshot(
  previous: MetricsDashboardSnapshot | null,
  identities: readonly MetricsSnapshotIdentity[],
  range: TimeRange,
  stepSeconds: number
): MetricsDashboardSnapshot {
  const series: Record<string, MetricsSeriesSnapshot> = {}
  for (const identity of identities) {
    const key = metricsSnapshotSeriesKey(identity)
    const points = emptyOrRetained(previous, key, range)
    series[key] = {
      status: points.length > 0 ? "stale" : "unavailable",
      points,
    }
  }

  return { range, stepSeconds, series }
}
