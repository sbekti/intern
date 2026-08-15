export type TimeSeriesPoint = readonly [timestamp: number, value: number]

export type TimeRange = {
  start: number
  end: number
}

export function timeRangeFraction(
  range: TimeRange | null,
  timestamp: number | null
) {
  if (
    !range ||
    timestamp === null ||
    range.end <= range.start ||
    timestamp < range.start ||
    timestamp > range.end
  ) {
    return null
  }

  return (timestamp - range.start) / (range.end - range.start)
}

export function timeRangeTimestampAtFraction(
  range: TimeRange | null,
  fraction: number,
  stepSeconds: number
) {
  if (
    !range ||
    range.end < range.start ||
    !Number.isFinite(fraction) ||
    !Number.isFinite(stepSeconds) ||
    stepSeconds <= 0
  ) {
    return null
  }

  const stepCount = Math.floor((range.end - range.start) / stepSeconds)
  const clampedFraction = Math.min(1, Math.max(0, fraction))
  return range.start + Math.round(clampedFraction * stepCount) * stepSeconds
}

type TimeTickInterval = {
  unit: "minute" | "hour" | "day" | "week"
  step: number
  seconds: number
}

const timeTickIntervals: readonly TimeTickInterval[] = [
  { unit: "minute", step: 1, seconds: 60 },
  { unit: "minute", step: 5, seconds: 5 * 60 },
  { unit: "minute", step: 15, seconds: 15 * 60 },
  { unit: "minute", step: 30, seconds: 30 * 60 },
  { unit: "hour", step: 1, seconds: 60 * 60 },
  { unit: "hour", step: 3, seconds: 3 * 60 * 60 },
  { unit: "hour", step: 6, seconds: 6 * 60 * 60 },
  { unit: "hour", step: 12, seconds: 12 * 60 * 60 },
  { unit: "day", step: 1, seconds: 24 * 60 * 60 },
  { unit: "day", step: 2, seconds: 2 * 24 * 60 * 60 },
  { unit: "week", step: 1, seconds: 7 * 24 * 60 * 60 },
]

function floorTimeTick(timestamp: number, interval: TimeTickInterval) {
  const date = new Date(timestamp * 1000)

  if (interval.unit === "minute") {
    date.setSeconds(0, 0)
    date.setMinutes(
      Math.floor(date.getMinutes() / interval.step) * interval.step
    )
  } else if (interval.unit === "hour") {
    date.setMinutes(0, 0, 0)
    date.setHours(Math.floor(date.getHours() / interval.step) * interval.step)
  } else if (interval.unit === "day") {
    date.setHours(0, 0, 0, 0)
    date.setDate(date.getDate() - ((date.getDate() - 1) % interval.step))
  } else {
    date.setHours(0, 0, 0, 0)
    date.setDate(date.getDate() - date.getDay())
  }

  return date.getTime() / 1000
}

function offsetTimeTick(timestamp: number, interval: TimeTickInterval) {
  const date = new Date(timestamp * 1000)

  if (interval.unit === "minute") {
    date.setMinutes(date.getMinutes() + interval.step)
  } else if (interval.unit === "hour") {
    date.setHours(date.getHours() + interval.step)
  } else if (interval.unit === "day") {
    date.setDate(date.getDate() + interval.step)
  } else {
    date.setDate(date.getDate() + 7)
  }

  return date.getTime() / 1000
}

export function timeRangeTicks(range: TimeRange | null, width: number) {
  if (
    !range ||
    range.end <= range.start ||
    !Number.isFinite(width) ||
    width <= 0
  ) {
    return []
  }

  const duration = range.end - range.start
  const targetCount = Math.min(12, Math.max(4, Math.floor(width / 80)))
  const interval = timeTickIntervals.reduce((closest, candidate) => {
    const closestDifference = Math.abs(duration / closest.seconds - targetCount)
    const candidateDifference = Math.abs(
      duration / candidate.seconds - targetCount
    )
    return candidateDifference <= closestDifference ? candidate : closest
  })

  const ticks: number[] = []
  let timestamp = floorTimeTick(range.start, interval)
  if (timestamp < range.start) {
    timestamp = offsetTimeTick(timestamp, interval)
  }

  while (timestamp <= range.end && ticks.length < 1000) {
    ticks.push(timestamp)
    timestamp = offsetTimeTick(timestamp, interval)
  }

  return ticks
}

export function clampedLabelPosition(
  desiredPosition: number,
  width: number,
  labelWidth: number
) {
  if (
    !Number.isFinite(desiredPosition) ||
    !Number.isFinite(width) ||
    !Number.isFinite(labelWidth) ||
    width <= 0
  ) {
    return 0
  }

  const halfWidth = Math.min(Math.max(0, labelWidth), width) / 2
  return Math.min(width - halfWidth, Math.max(halfWidth, desiredPosition))
}

export function overlappingTickIndexes(
  tickPositions: readonly number[],
  focusPosition: number | null,
  focusLabelWidth: number,
  gap = 6
) {
  if (focusPosition === null || focusLabelWidth <= 0) {
    return []
  }

  return tickPositions.flatMap((position, index) =>
    Math.abs(position - focusPosition) < focusLabelWidth + gap ? [index] : []
  )
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
