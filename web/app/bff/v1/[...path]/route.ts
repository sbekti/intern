import type { NextRequest } from "next/server"

import { buildForwardHeaders, resolveApiBaseUrl } from "@/lib/api"

async function buildProxyHeaders(request: NextRequest, hasBody: boolean) {
  const outbound = await buildForwardHeaders(request.headers)
  if (hasBody) {
    const contentType = request.headers.get("content-type")

    if (contentType) {
      outbound.set("content-type", contentType)
    }
  }

  return outbound
}

async function proxy(request: NextRequest, path: string[]) {
  const baseUrl = await resolveApiBaseUrl()
  const target = `${baseUrl}/api/v1/${path.join("/")}${request.nextUrl.search}`
  const hasBody = request.method !== "GET" && request.method !== "HEAD"
  const body = hasBody ? await request.text() : undefined

  const response = await fetch(target, {
    method: request.method,
    headers: await buildProxyHeaders(request, hasBody),
    body,
    redirect: "manual",
    cache: "no-store",
  })

  const headers = new Headers()
  const contentType = response.headers.get("content-type")

  if (contentType) {
    headers.set("content-type", contentType)
  }

  return new Response(response.body, {
    status: response.status,
    headers,
  })
}

type RouteContext = {
  params: Promise<{
    path: string[]
  }>
}

export async function GET(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function PATCH(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
