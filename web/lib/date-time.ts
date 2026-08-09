function parseTimestamp(value: string) {
  const timestamp = new Date(value)
  return Number.isNaN(timestamp.getTime()) ? null : timestamp
}

export function formatCompactLocalTimestamp(value: string) {
  const timestamp = parseTimestamp(value)

  if (!timestamp) {
    return value
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(timestamp)
}

export function formatExactLocalTimestamp(value: string) {
  const timestamp = parseTimestamp(value)

  if (!timestamp) {
    return value
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "full",
    timeStyle: "long",
  }).format(timestamp)
}
