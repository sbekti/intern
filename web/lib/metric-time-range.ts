export const metricTimePresets = [
  { id: "1h", durationSeconds: 60 * 60, stepSeconds: 10 },
  { id: "6h", durationSeconds: 6 * 60 * 60, stepSeconds: 10 },
  { id: "24h", durationSeconds: 24 * 60 * 60, stepSeconds: 60 },
  { id: "3d", durationSeconds: 3 * 24 * 60 * 60, stepSeconds: 300 },
  { id: "7d", durationSeconds: 7 * 24 * 60 * 60, stepSeconds: 300 },
] as const

export type MetricTimePreset = (typeof metricTimePresets)[number]
export type MetricTimePresetId = MetricTimePreset["id"]

export const defaultMetricTimePreset = metricTimePresets[1]
export const metricTimeSteps = [
  ...new Set(metricTimePresets.map(({ stepSeconds }) => stepSeconds)),
]

export function metricTimePreset(value: unknown): MetricTimePreset {
  return (
    metricTimePresets.find(({ id }) => id === value) ?? defaultMetricTimePreset
  )
}

export function metricTimeRangePath(
  href: string,
  presetId: MetricTimePresetId
) {
  const url = new URL(href)
  if (presetId === defaultMetricTimePreset.id) {
    url.searchParams.delete("range")
  } else {
    url.searchParams.set("range", presetId)
  }

  return `${url.pathname}${url.search}${url.hash}`
}
