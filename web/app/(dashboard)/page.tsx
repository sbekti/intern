import { connection } from "next/server"

import { HomeMetricsDashboard } from "@/components/home-metrics-card"
import { clientMetricGroups } from "@/lib/metrics-config"
import { readMetricsConfig } from "@/lib/metrics-config-server"
import { createPageMetadata } from "@/lib/page-titles"

export const metadata = createPageMetadata("/")

export default async function HomePage() {
  await connection()
  const config = readMetricsConfig()

  return (
    <div className="px-4 lg:px-6">
      <HomeMetricsDashboard
        groups={config ? clientMetricGroups(config) : null}
      />
    </div>
  )
}
