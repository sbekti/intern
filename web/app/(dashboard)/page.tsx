import { connection } from "next/server"

import { HomeMetricsDashboard } from "@/components/home-metrics-card"
import { metricTimePreset } from "@/lib/metric-time-range"
import { clientMetricGroups } from "@/lib/metrics-config"
import { readMetricsConfig } from "@/lib/metrics-config-server"
import { createPageMetadata } from "@/lib/page-titles"

export const metadata = createPageMetadata("/")

type SearchParams = Promise<Record<string, string | string[] | undefined>>

export default async function HomePage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  await connection()
  const params = await searchParams
  const config = readMetricsConfig()
  const initialPreset = metricTimePreset(params.range)

  return (
    <div className="px-4 lg:px-6">
      <HomeMetricsDashboard
        key={initialPreset.id}
        groups={config ? clientMetricGroups(config) : null}
        initialPresetId={initialPreset.id}
      />
    </div>
  )
}
