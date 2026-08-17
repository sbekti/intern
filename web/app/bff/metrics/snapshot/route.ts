import type { NextRequest } from "next/server"
import { NextResponse } from "next/server"

import { getMetricsSnapshot } from "@/lib/metrics-snapshot"
import { readMetricsConfig } from "@/lib/metrics-config-server"

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

  const baseUrl = prometheusBaseUrl()
  if (!baseUrl) {
    return errorResponse("Metrics service unavailable.", 503)
  }

  const result = await getMetricsSnapshot(
    request.nextUrl.searchParams,
    config,
    baseUrl,
    { signal: request.signal }
  )
  if (!result.ok) {
    return errorResponse(result.message, result.status)
  }

  return NextResponse.json(result.data, {
    headers: { "Cache-Control": "no-store" },
  })
}
