import { HomeMetricsCard } from "@/components/home-metrics-card"
import { createPageMetadata } from "@/lib/page-titles"

export const metadata = createPageMetadata("/")

export default function HomePage() {
  return (
    <div className="px-4 lg:px-6">
      <HomeMetricsCard />
    </div>
  )
}
