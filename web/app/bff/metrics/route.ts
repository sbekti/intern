import type { NextRequest } from "next/server"
import { NextResponse } from "next/server"

import { parseBoundedTimeRange } from "@/components/live-horizon/model"
import { findMetricSeries } from "@/lib/metrics-config"
import { readMetricsConfig } from "@/lib/metrics-config-server"
import { parsePrometheusMatrix, parsePrometheusStep } from "@/lib/prometheus"

const allowedSteps = [10, 30, 60, 300] as const
const maxSteps = 4096
const upstreamTimeoutMs = 5_000

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

export async function GET(request: NextRequest) {
  const config = readMetricsConfig()
  if (!config) {
    return errorResponse("Metrics service unavailable.", 503)
  }

  const query = findMetricSeries(
    config,
    request.nextUrl.searchParams.get("group"),
    request.nextUrl.searchParams.get("lane"),
    request.nextUrl.searchParams.get("series")
  )?.promql
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
