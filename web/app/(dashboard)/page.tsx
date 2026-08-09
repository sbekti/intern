import { CloudSunIcon } from "lucide-react"

import { UnauthorizedState } from "@/components/api-state"
import { HomeLoadingPanel } from "@/components/loading-panels"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { getDashboard } from "@/lib/api"
import { createPageMetadata } from "@/lib/page-titles"
import { hasForcedGlimmer } from "@/lib/utils"

type SearchParams = Promise<Record<string, string | string[] | undefined>>

export const metadata = createPageMetadata("/")

function weatherCodeLabel(code: number) {
  switch (code) {
    case 0:
      return "Clear"
    case 1:
    case 2:
    case 3:
      return "Partly cloudy"
    case 45:
    case 48:
      return "Fog"
    case 51:
    case 53:
    case 55:
    case 61:
    case 63:
    case 65:
      return "Rain"
    case 71:
    case 73:
    case 75:
      return "Snow"
    case 95:
    case 96:
    case 99:
      return "Thunderstorm"
    default:
      return `Code ${code}`
  }
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: SearchParams
}) {
  const params = await searchParams

  if (hasForcedGlimmer(params)) {
    return <HomeLoadingPanel />
  }

  const dashboard = await getDashboard()

  if (!dashboard.ok) {
    return <UnauthorizedState />
  }

  return (
    <div className="grid gap-4 px-4 lg:px-6">
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <CardTitle>Welcome to Bektinet</CardTitle>
          <CardDescription>
            {dashboard.data.welcome_message}
          </CardDescription>
        </CardHeader>
      </Card>
      <Card className="border-border/70 shadow-xs">
        <CardHeader>
          <div className="flex items-center gap-2">
            <CloudSunIcon />
            <CardTitle>Weather</CardTitle>
          </div>
          <CardDescription>
            {dashboard.data.weather
              ? `${dashboard.data.weather.location_name} · ${dashboard.data.weather.timezone}`
              : "No weather data returned by the dashboard endpoint."}
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm leading-6 text-muted-foreground">
          {dashboard.data.weather ? (
            <div className="grid gap-4 sm:grid-cols-3">
              <div className="rounded-xl border bg-muted/30 p-4">
                <p className="text-sm font-medium text-foreground">Condition</p>
                <p className="mt-2 text-sm">
                  {weatherCodeLabel(dashboard.data.weather.current.weather_code)}
                </p>
              </div>
              <div className="rounded-xl border bg-muted/30 p-4">
                <p className="text-sm font-medium text-foreground">
                  Temperature
                </p>
                <p className="mt-2 text-sm">
                  {dashboard.data.weather.current.temperature_c.toFixed(1)} C
                </p>
              </div>
              <div className="rounded-xl border bg-muted/30 p-4">
                <p className="text-sm font-medium text-foreground">Wind</p>
                <p className="mt-2 text-sm">
                  {dashboard.data.weather.current.wind_speed_kph.toFixed(1)} kph
                </p>
              </div>
            </div>
          ) : (
            "The backend is reachable, but no weather payload is currently available."
          )}
        </CardContent>
      </Card>
    </div>
  )
}
