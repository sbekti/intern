export type TimeSeriesPoint = readonly [timestamp: number, value: number]

export type TimeRange = {
  start: number
  end: number
}

export type TimeSeriesLoader = (
  range: TimeRange,
  signal: AbortSignal
) => Promise<readonly TimeSeriesPoint[]>

export type TimeSeriesPayload = TimeRange & {
  step_seconds: number
  points: TimeSeriesPoint[]
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function parseUnixSeconds(value: string | null) {
  if (value === null || !/^\d+$/.test(value)) {
    return null
  }

  const parsed = Number(value)
  return Number.isSafeInteger(parsed) ? parsed : null
}

export function parseBoundedTimeRange(
  params: URLSearchParams,
  stepSeconds: number,
  maxSteps: number
): TimeRange | null {
  const start = parseUnixSeconds(params.get("start"))
  const end = parseUnixSeconds(params.get("end"))

  if (start === null || end === null || end < start) {
    return null
  }

  const steps = Math.floor((end - start) / stepSeconds) + 1
  return steps <= maxSteps ? { start, end } : null
}

export function mergeTimeSeriesPoints(
  current: readonly TimeSeriesPoint[],
  incoming: readonly TimeSeriesPoint[],
  minimumTimestamp: number
) {
  const merged = new Map<number, number>()

  for (const [timestamp, value] of current) {
    if (timestamp >= minimumTimestamp) {
      merged.set(timestamp, value)
    }
  }

  for (const [timestamp, value] of incoming) {
    if (timestamp >= minimumTimestamp) {
      merged.set(timestamp, value)
    }
  }

  return [...merged.entries()]
    .sort(([left], [right]) => left - right)
    .map(([timestamp, value]) => [timestamp, value] as const)
}

export function timeSeriesExtent(
  points: readonly TimeSeriesPoint[],
  headroom = 0
): readonly [minimum: number, maximum: number] {
  if (points.length === 0) {
    return [-1, 1]
  }

  let minimum = Infinity
  let maximum = -Infinity
  for (const [, value] of points) {
    minimum = Math.min(minimum, value)
    maximum = Math.max(maximum, value)
  }

  const multiplier = 1 + Math.max(0, Number.isFinite(headroom) ? headroom : 0)
  const magnitude =
    (Math.max(Math.abs(minimum), Math.abs(maximum)) || 1) * multiplier
  return [minimum < 0 ? -magnitude : 0, maximum > 0 ? magnitude : 0]
}

export function isTimeSeriesPayload(
  input: unknown,
  expectedStepSeconds: number
): input is TimeSeriesPayload {
  if (!isRecord(input) || !Array.isArray(input.points)) {
    return false
  }

  return (
    input.step_seconds === expectedStepSeconds &&
    typeof input.start === "number" &&
    Number.isFinite(input.start) &&
    typeof input.end === "number" &&
    Number.isFinite(input.end) &&
    input.points.every(
      (point) =>
        Array.isArray(point) &&
        point.length === 2 &&
        typeof point[0] === "number" &&
        Number.isFinite(point[0]) &&
        typeof point[1] === "number" &&
        Number.isFinite(point[1])
    )
  )
}
