export const metricFormats = [
  "percent",
  "bits-per-second",
  "count",
  "requests-per-second",
  "duration-seconds",
] as const
export type MetricFormat = (typeof metricFormats)[number]

export type MetricSeriesSide = "top" | "bottom"

export type MetricSeries = {
  id: string
  label: string
  side?: MetricSeriesSide
  promql: string
}

export type MetricLane = {
  id: string
  label: string
  format: MetricFormat
  extent: readonly [minimum: number, maximum: number]
  series: readonly MetricSeries[]
}

export type MetricGroup = {
  id: string
  title: string
  subtitle?: string
  lanes: readonly MetricLane[]
}

export type MetricsConfig = {
  groups: readonly MetricGroup[]
}

export type ClientMetricSeries = Omit<MetricSeries, "promql">
export type ClientMetricLane = Omit<MetricLane, "series"> & {
  series: readonly ClientMetricSeries[]
}
export type ClientMetricGroup = Omit<MetricGroup, "lanes"> & {
  lanes: readonly ClientMetricLane[]
}

const idPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function nonEmptyString(value: unknown) {
  return typeof value === "string" && value.trim() ? value.trim() : null
}

function metricId(value: unknown) {
  const id = nonEmptyString(value)
  return id && idPattern.test(id) ? id : null
}

function uniqueIds(values: readonly { id: string }[]) {
  return new Set(values.map(({ id }) => id)).size === values.length
}

function parseSeries(value: unknown): MetricSeries | null {
  if (!isRecord(value)) {
    return null
  }

  const id = metricId(value.id)
  const label = nonEmptyString(value.label)
  const promql = nonEmptyString(value.promql)
  const side =
    value.side === "top" || value.side === "bottom" ? value.side : undefined

  if (!id || !label || !promql || (value.side !== undefined && !side)) {
    return null
  }

  return { id, label, promql, ...(side ? { side } : {}) }
}

function parseLane(value: unknown): MetricLane | null {
  if (
    !isRecord(value) ||
    !Array.isArray(value.extent) ||
    value.extent.length !== 2 ||
    !Array.isArray(value.series)
  ) {
    return null
  }

  const id = metricId(value.id)
  const label = nonEmptyString(value.label)
  const format = metricFormats.find((candidate) => candidate === value.format)
  const [minimum, maximum] = value.extent
  const series = value.series.map(parseSeries)

  if (
    !id ||
    !label ||
    !format ||
    typeof minimum !== "number" ||
    !Number.isFinite(minimum) ||
    typeof maximum !== "number" ||
    !Number.isFinite(maximum) ||
    minimum < 0 ||
    maximum <= minimum ||
    series.some((candidate) => candidate === null)
  ) {
    return null
  }

  const parsedSeries = series as MetricSeries[]
  if (
    parsedSeries.length < 1 ||
    parsedSeries.length > 2 ||
    !uniqueIds(parsedSeries)
  ) {
    return null
  }

  if (parsedSeries.length === 1 && parsedSeries[0].side !== undefined) {
    return null
  }
  if (
    parsedSeries.length === 2 &&
    (!parsedSeries.some(({ side }) => side === "top") ||
      !parsedSeries.some(({ side }) => side === "bottom"))
  ) {
    return null
  }

  const orderedSeries =
    parsedSeries.length === 2
      ? [...parsedSeries].sort((left, right) =>
          left.side === right.side ? 0 : left.side === "top" ? -1 : 1
        )
      : parsedSeries

  return {
    id,
    label,
    format,
    extent: [minimum, maximum],
    series: orderedSeries,
  }
}

function parseGroup(value: unknown): MetricGroup | null {
  if (!isRecord(value) || !Array.isArray(value.lanes)) {
    return null
  }

  const id = metricId(value.id)
  const title = nonEmptyString(value.title)
  const subtitle =
    value.subtitle === undefined ? undefined : nonEmptyString(value.subtitle)
  const lanes = value.lanes.map(parseLane)

  if (
    !id ||
    !title ||
    (value.subtitle !== undefined && !subtitle) ||
    lanes.length === 0 ||
    lanes.some((candidate) => candidate === null)
  ) {
    return null
  }

  const parsedLanes = lanes as MetricLane[]
  if (!uniqueIds(parsedLanes)) {
    return null
  }

  return {
    id,
    title,
    ...(subtitle ? { subtitle } : {}),
    lanes: parsedLanes,
  }
}

export function parseMetricsConfig(input: unknown) {
  if (!isRecord(input) || !Array.isArray(input.groups)) {
    return null
  }

  const groups = input.groups.map(parseGroup)
  if (groups.length === 0 || groups.some((candidate) => candidate === null)) {
    return null
  }

  const parsedGroups = groups as MetricGroup[]
  return uniqueIds(parsedGroups) ? { groups: parsedGroups } : null
}

export function clientMetricGroups(config: MetricsConfig): ClientMetricGroup[] {
  return config.groups.map((group) => ({
    ...group,
    lanes: group.lanes.map((lane) => ({
      ...lane,
      series: lane.series.map(({ id, label, side }) => ({
        id,
        label,
        ...(side ? { side } : {}),
      })),
    })),
  }))
}

function significantNumber(value: number, minimumSignificantDigits = 1) {
  return new Intl.NumberFormat(undefined, {
    minimumSignificantDigits,
    maximumSignificantDigits: 2,
    useGrouping: false,
  }).format(value)
}

export function formatMetricValue(value: number, format: MetricFormat) {
  if (format === "percent") {
    return `${value.toFixed(1)}%`
  }

  if (format === "count") {
    return Math.round(value).toString()
  }

  if (format === "requests-per-second") {
    return `${significantNumber(value)} RPS`
  }

  if (format === "duration-seconds") {
    const milliseconds = Math.round(value * 1000)
    if (Math.abs(milliseconds) < 1000) {
      return `${milliseconds} ms`
    }

    return `${significantNumber(value)} s`
  }

  if (value === 0) {
    return "0 bps"
  }

  const units = ["bps", "kbps", "Mbps", "Gbps", "Tbps"] as const
  let scaled = Math.abs(value)
  let unit = 0
  while (scaled >= 1000 && unit < units.length - 1) {
    scaled /= 1000
    unit += 1
  }

  scaled = Number(scaled.toPrecision(2))
  if (scaled >= 1000 && unit < units.length - 1) {
    scaled /= 1000
    unit += 1
  }

  const signed = value < 0 ? -scaled : scaled
  const formatted = significantNumber(signed, 2)
  return `${formatted} ${units[unit]}`
}
