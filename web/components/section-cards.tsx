"use client"

import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardAction,
  CardHeader,
  CardDescription,
  CardTitle,
} from "@/components/ui/card"
import {
  CloudSunIcon,
  Layers3Icon,
  ShieldCheckIcon,
  SmartphoneNfcIcon,
} from "lucide-react"

export function SectionCards() {
  return (
    <div className="grid grid-cols-1 gap-4 px-4 *:data-[slot=card]:bg-linear-to-t *:data-[slot=card]:from-primary/5 *:data-[slot=card]:to-card *:data-[slot=card]:shadow-xs lg:px-6 @xl/main:grid-cols-2 @5xl/main:grid-cols-4 dark:*:data-[slot=card]:bg-card">
      <Card className="@container/card">
        <CardHeader>
          <CardDescription>Authentication</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
            Forward Auth
          </CardTitle>
          <CardAction>
            <Badge variant="outline">
              <ShieldCheckIcon />
              trusted ingress
            </Badge>
          </CardAction>
        </CardHeader>
      </Card>
      <Card className="@container/card">
        <CardHeader>
          <CardDescription>Network Scope</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
            Global
          </CardTitle>
          <CardAction>
            <Badge variant="outline">
              <Layers3Icon />
              v1
            </Badge>
          </CardAction>
        </CardHeader>
      </Card>
      <Card className="@container/card">
        <CardHeader>
          <CardDescription>Client Access</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
            Device Code
          </CardTitle>
          <CardAction>
            <Badge variant="outline">
              <SmartphoneNfcIcon />
              cli + mobile ready
            </Badge>
          </CardAction>
        </CardHeader>
      </Card>
      <Card className="@container/card">
        <CardHeader>
          <CardDescription>Dashboard</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
            Weather
          </CardTitle>
          <CardAction>
            <Badge variant="outline">
              <CloudSunIcon />
              queued
            </Badge>
          </CardAction>
        </CardHeader>
      </Card>
    </div>
  )
}
