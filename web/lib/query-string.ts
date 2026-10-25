export function withQuery(
  path: string,
  values: Record<string, string | number | null | undefined>
) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null && value !== "") {
      params.set(key, String(value))
    }
  }
  const query = params.toString()
  return query ? `${path}?${query}` : path
}
