import type { NextRequest } from "next/server"
import { NextResponse } from "next/server"

import { parseBoundedTimeRange } from "@/components/live-horizon/model"
import { parsePrometheusMatrix, parsePrometheusStep } from "@/lib/prometheus"

const allowedSteps = [30, 60, 300] as const
const maxSteps = 4096
const upstreamTimeoutMs = 5_000
const metricQueries = {
  cpu: '100 * (1 - avg(rate(node_cpu_seconds_total{job="node-exporter",mode="idle"}[5m])))',
  memory:
    '100 * (1 - sum(node_memory_MemAvailable_bytes{job="node-exporter"}) / sum(node_memory_MemTotal_bytes{job="node-exporter"}))',
} as const

function errorResponse(message: string, status: number) {
  return NextResponse.json(
    { error: message },
    { status, headers: { "Cache-Control": "no-store" } }
  )
}

function prometheusBaseUrl() {
  const value = process.env.INTERN_PROMETHEUS_BASE_URL?.trim()
  if (!value) {
    return null
  }

  try {
    return new URL(value.endsWith("/") ? value : `${value}/`)
  } catch {
    return null
  }
}

function metricQuery(name: string | null) {
  return name && Object.hasOwn(metricQueries, name)
    ? metricQueries[name as keyof typeof metricQueries]
    : null
}

export async function GET(request: NextRequest) {
  const query = metricQuery(request.nextUrl.searchParams.get("metric"))
  if (!query) {
    return errorResponse("Unknown metric.", 400)
  }

  const stepSeconds = parsePrometheusStep(
    request.nextUrl.searchParams.get("step"),
    allowedSteps
  )
  if (!stepSeconds) {
    return errorResponse("Invalid metric step.", 400)
  }

  const range = parseBoundedTimeRange(
    request.nextUrl.searchParams,
    stepSeconds,
    maxSteps
  )
  if (!range) {
    return errorResponse("Invalid metric range.", 400)
  }

  const baseUrl = prometheusBaseUrl()
  if (!baseUrl) {
    return errorResponse("Metrics service unavailable.", 503)
  }

  const target = new URL("api/v1/query_range", baseUrl)
  target.searchParams.set("query", query)
  target.searchParams.set("start", String(range.start))
  target.searchParams.set("end", String(range.end))
  target.searchParams.set("step", String(stepSeconds))
  target.searchParams.set("limit", "1")
  target.searchParams.set("timeout", "4s")

  let response: Response
  try {
    response = await fetch(target, {
      cache: "no-store",
      signal: AbortSignal.timeout(upstreamTimeoutMs),
    })
  } catch {
    return errorResponse("Metrics service unavailable.", 503)
  }

  if (!response.ok) {
    return errorResponse("Metrics query failed.", 502)
  }

  let body: unknown
  try {
    body = await response.json()
  } catch {
    return errorResponse("Metrics query failed.", 502)
  }

  const points = parsePrometheusMatrix(body)
  if (!points) {
    return errorResponse("Metrics query returned no usable data.", 502)
  }

  return NextResponse.json(
    {
      step_seconds: stepSeconds,
      start: range.start,
      end: range.end,
      points,
    },
    { headers: { "Cache-Control": "no-store" } }
  )
}
