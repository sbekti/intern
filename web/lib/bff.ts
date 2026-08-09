const browserApiPrefix = "/bff/v1"

export function buildBffPath(path: string) {
  if (path.startsWith("/")) {
    return `${browserApiPrefix}${path}`
  }

  return `${browserApiPrefix}/${path}`
}
