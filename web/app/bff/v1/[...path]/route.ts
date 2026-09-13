import type { NextRequest } from "next/server"

import { buildForwardHeaders, resolveApiBaseUrl } from "@/lib/api"

const maxRequestBodyBytes = 1 << 20

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

async function readRequestBody(request: NextRequest) {
  const contentLength = Number(request.headers.get("content-length"))
  if (contentLength > maxRequestBodyBytes) {
    return null
  }

  const reader = request.body?.getReader()
  if (!reader) {
    return undefined
  }

  const chunks: Uint8Array[] = []
  let length = 0

  while (true) {
    const { done, value } = await reader.read()
    if (done) {
      break
    }
    length += value.byteLength
    if (length > maxRequestBodyBytes) {
      await reader.cancel()
      return null
    }
    chunks.push(value)
  }

  const body = new Uint8Array(length)
  let offset = 0
  for (const chunk of chunks) {
    body.set(chunk, offset)
    offset += chunk.byteLength
  }
  return body
}

async function proxy(request: NextRequest, path: string[]) {
  const baseUrl = await resolveApiBaseUrl()
  const target = `${baseUrl}/api/v1/${path.join("/")}${request.nextUrl.search}`
  const hasBody = request.method !== "GET" && request.method !== "HEAD"
  const body = hasBody ? await readRequestBody(request) : undefined

  if (body === null) {
    return Response.json(
      { code: "request_too_large", message: "request body too large" },
      { status: 413 }
    )
  }

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

export async function PUT(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
