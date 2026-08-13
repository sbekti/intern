import type { TimeSeriesPoint } from "../components/live-horizon/model"

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

export function parsePrometheusStep(
  value: string | null,
  allowedSteps: readonly number[]
) {
  if (value === null || !/^\d+$/.test(value)) {
    return null
  }

  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && allowedSteps.includes(parsed)
    ? parsed
    : null
}

export function parsePrometheusMatrix(
  input: unknown,
  expectedSeries = 1
): TimeSeriesPoint[] | null {
  if (!isRecord(input) || input.status !== "success" || !isRecord(input.data)) {
    return null
  }

  const { data } = input
  if (
    data.resultType !== "matrix" ||
    !Array.isArray(data.result) ||
    data.result.length !== expectedSeries
  ) {
    return null
  }

  const series = data.result[0]
  if (
    !isRecord(series) ||
    !Array.isArray(series.values) ||
    series.values.length === 0
  ) {
    return null
  }

  const points: TimeSeriesPoint[] = []
  let previousTimestamp = -Infinity

  for (const sample of series.values) {
    if (!Array.isArray(sample) || sample.length < 2) {
      return null
    }

    const timestamp = sample[0]
    const value = typeof sample[1] === "string" ? Number(sample[1]) : NaN

    if (
      typeof timestamp !== "number" ||
      !Number.isFinite(timestamp) ||
      !Number.isFinite(value) ||
      timestamp <= previousTimestamp
    ) {
      return null
    }

    points.push([timestamp, value])
    previousTimestamp = timestamp
  }

  return points
}
