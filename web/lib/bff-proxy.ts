const browserApiResources = new Set([
  "admin",
  "auth",
  "gizmos",
  "networks",
  "profile",
])

export function resolveBffTarget(
  baseUrl: string,
  path: string[],
  search: string
) {
  const apiBase = new URL(`${baseUrl}/api/v1/`)
  const target = new URL(path.map(encodeURIComponent).join("/"), apiBase)

  if (
    target.origin !== apiBase.origin ||
    !target.pathname.startsWith(apiBase.pathname)
  ) {
    return null
  }

  const resource = target.pathname.slice(apiBase.pathname.length).split("/")[0]
  if (!browserApiResources.has(resource)) {
    return null
  }

  target.search = search
  return target
}
