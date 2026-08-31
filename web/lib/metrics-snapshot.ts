import { parseBoundedTimeRange } from "../components/live-horizon/model.ts"
import { metricTimeSteps } from "./metric-time-range.ts"
import type {
  MetricsSnapshot,
  MetricsSnapshotSeries,
} from "./metrics-snapshot-client.ts"
import type { MetricsConfig } from "./metrics-config.ts"
import { parsePrometheusMatrix, parsePrometheusStep } from "./prometheus.ts"

const maxSteps = 4096
const upstreamTimeoutMs = 5_000

type ConfiguredSeries = {
  group: string
  lane: string
  series: string
  promql: string
}

type SnapshotSeriesIdentity = Pick<
  ConfiguredSeries,
  "group" | "lane" | "series"
>

export type MetricsSnapshotResult =
  | { ok: true; data: MetricsSnapshot }
  | { ok: false; status: 400; message: string }

type Fetcher = typeof fetch

type MetricsSnapshotOptions = {
  fetcher?: Fetcher
  signal?: AbortSignal
}

function configuredSeries(config: MetricsConfig): ConfiguredSeries[] {
  return config.groups.flatMap((group) =>
    group.lanes.flatMap((lane) =>
      lane.series.map((series) => ({
        group: group.id,
        lane: lane.id,
        series: series.id,
        promql: series.promql,
      }))
    )
  )
}

function hasOnlySnapshotParameters(params: URLSearchParams) {
  const allowed = new Set(["start", "end", "step"])
  return [...params.keys()].every((key) => allowed.has(key))
}

function errorSeries(series: SnapshotSeriesIdentity): MetricsSnapshotSeries {
  return { ...series, status: "error" }
}

async function querySeries(
  baseUrl: URL,
  series: ConfiguredSeries,
  start: number,
  end: number,
  stepSeconds: number,
  fetcher: Fetcher,
  requestSignal?: AbortSignal
): Promise<MetricsSnapshotSeries> {
  const identity: SnapshotSeriesIdentity = {
    group: series.group,
    lane: series.lane,
    series: series.series,
  }
  const target = new URL("api/v1/query_range", baseUrl)
  target.searchParams.set("query", series.promql)
  target.searchParams.set("start", String(start))
  target.searchParams.set("end", String(end))
  target.searchParams.set("step", String(stepSeconds))
  target.searchParams.set("limit", "1")
  target.searchParams.set("timeout", "4s")

  try {
    const timeoutSignal = AbortSignal.timeout(upstreamTimeoutMs)
    const response = await fetcher(target, {
      cache: "no-store",
      signal: requestSignal
        ? AbortSignal.any([requestSignal, timeoutSignal])
        : timeoutSignal,
    })

    if (!response.ok) {
      return errorSeries(identity)
    }

    let body: unknown
    try {
      body = await response.json()
    } catch {
      return errorSeries(identity)
    }

    const points = parsePrometheusMatrix(body)
    return points
      ? { ...identity, status: "ok", points }
      : errorSeries(identity)
  } catch {
    return errorSeries(identity)
  }
}

export async function getMetricsSnapshot(
  params: URLSearchParams,
  config: MetricsConfig,
  baseUrl: URL,
  { fetcher = fetch, signal }: MetricsSnapshotOptions = {}
): Promise<MetricsSnapshotResult> {
  if (!hasOnlySnapshotParameters(params)) {
    return { ok: false, status: 400, message: "Invalid metric request." }
  }

  const stepSeconds = parsePrometheusStep(params.get("step"), metricTimeSteps)
  if (!stepSeconds) {
    return { ok: false, status: 400, message: "Invalid metric step." }
  }

  const range = parseBoundedTimeRange(params, stepSeconds, maxSteps)
  if (!range) {
    return { ok: false, status: 400, message: "Invalid metric range." }
  }

  const series = configuredSeries(config)
  const results = await Promise.all(
    series.map((configured) =>
      querySeries(
        baseUrl,
        configured,
        range.start,
        range.end,
        stepSeconds,
        fetcher,
        signal
      )
    )
  )

  return {
    ok: true,
    data: {
      version: 1,
      start: range.start,
      end: range.end,
      step_seconds: stepSeconds,
      series: results,
    },
  }
}
