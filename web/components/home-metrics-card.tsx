"use client"

import { useCallback, useMemo, useState } from "react"

import {
  isTimeSeriesPayload,
  LiveHorizon,
  type LiveHorizonSnapshot,
  type LiveHorizonStatus,
  type TimeRange,
  type TimeSeriesLoader,
  type TimeSeriesPoint,
} from "@/components/live-horizon"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { cn } from "@/lib/utils"

const timePresets = [
  { id: "1h", durationSeconds: 60 * 60, stepSeconds: 30 },
  { id: "6h", durationSeconds: 6 * 60 * 60, stepSeconds: 30 },
  { id: "24h", durationSeconds: 24 * 60 * 60, stepSeconds: 60 },
  { id: "7d", durationSeconds: 7 * 24 * 60 * 60, stepSeconds: 300 },
] as const
type TimePreset = (typeof timePresets)[number]
const defaultTimePreset = timePresets[1]
const positiveColors = [
  "var(--horizon-positive-1)",
  "var(--horizon-positive-2)",
  "var(--horizon-positive-3)",
  "var(--horizon-positive-4)",
] as const
const previewExtent = [0, 100] as const
const timeFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "numeric",
  minute: "2-digit",
})
const dateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "numeric",
})

const emptySnapshot: LiveHorizonSnapshot = {
  status: "loading",
  latestPoint: null,
  rulerPoint: null,
  range: null,
}

function metricLoader(
  metric: "cpu" | "memory",
  stepSeconds: number
): TimeSeriesLoader {
  return async (range, signal) => {
    const query = new URLSearchParams({
      metric,
      step: String(stepSeconds),
      start: String(range.start),
      end: String(range.end),
    })
    const response = await fetch(`/bff/metrics/node?${query}`, {
      cache: "no-store",
      signal,
    })

    if (!response.ok) {
      throw new Error(`${metric} metric request failed`)
    }

    const body: unknown = await response.json()
    if (!isTimeSeriesPayload(body, stepSeconds)) {
      throw new Error(`${metric} metric response was invalid`)
    }

    return body.points
  }
}

function bandPreviewLoader(stepSeconds: number): TimeSeriesLoader {
  const values = [25, 50, 75, 100] as const

  return async ({ start, end }) => {
    const points: TimeSeriesPoint[] = []
    let index = 0

    for (let timestamp = start; timestamp <= end; timestamp += stepSeconds) {
      points.push([timestamp, values[index % values.length]])
      index += 1
    }

    return points
  }
}

function badgeVariant(status: LiveHorizonStatus) {
  if (status === "live") {
    return "outline" as const
  }
  if (status === "loading") {
    return "secondary" as const
  }
  return "destructive" as const
}

function statusLabel(status: LiveHorizonStatus) {
  if (status === "loading") {
    return "Loading"
  }
  if (status === "unavailable") {
    return "Unavailable"
  }
  return status === "live" ? "Live" : "Stale"
}

function combinedStatus(snapshots: readonly LiveHorizonSnapshot[]) {
  if (snapshots.some(({ status }) => status === "unavailable")) {
    return "unavailable" as const
  }
  if (snapshots.some(({ status }) => status === "stale")) {
    return "stale" as const
  }
  if (snapshots.some(({ status }) => status === "loading")) {
    return "loading" as const
  }
  return "live" as const
}

function formatValue(value: number) {
  return `${value.toFixed(1)}%`
}

function MetricLane({
  label,
  load,
  snapshot,
  onStateChange,
  rulerTimestamp,
  onRulerTimestampChange,
  preset,
  extent,
}: {
  label: string
  load: TimeSeriesLoader
  snapshot: LiveHorizonSnapshot
  onStateChange: (snapshot: LiveHorizonSnapshot) => void
  rulerTimestamp: number | null
  onRulerTimestampChange: (timestamp: number | null) => void
  preset: TimePreset
  extent?: readonly [minimum: number, maximum: number]
}) {
  const point = snapshot.rulerPoint ?? snapshot.latestPoint

  return (
    <div className="relative">
      <LiveHorizon
        key={`${label}-${preset.id}`}
        load={load}
        stepSeconds={preset.stepSeconds}
        windowSeconds={preset.durationSeconds}
        ariaLabel={`Live IAD2 ${label.toLowerCase()} horizon chart.`}
        className="border-y border-border/60 bg-muted/20"
        rulerTimestamp={rulerTimestamp}
        onRulerTimestampChange={onRulerTimestampChange}
        onStateChange={onStateChange}
        positiveColors={positiveColors}
        extent={extent}
        extentHeadroom={0.5}
        height={64}
      />
      <div className="pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between gap-4 p-2 text-sm">
        <span className="rounded-sm bg-background/80 px-1.5 py-0.5 font-medium backdrop-blur-xs">
          {label}
        </span>
        <output className="rounded-sm bg-background/80 px-1.5 py-0.5 font-semibold tabular-nums backdrop-blur-xs">
          {point ? formatValue(point[1]) : "—"}
        </output>
      </div>
    </div>
  )
}

function TimeAxis({
  range,
  durationSeconds,
}: {
  range: TimeRange | null
  durationSeconds: number
}) {
  const ticks = [0, 0.25, 0.5, 0.75, 1]

  return (
    <div className="grid grid-cols-5 text-[11px] text-muted-foreground tabular-nums">
      {ticks.map((fraction, index) => {
        const timestamp = range
          ? range.start + (range.end - range.start) * fraction
          : null
        return (
          <span
            key={fraction}
            className={cn(
              "text-center",
              index === 0 && "text-left",
              index === ticks.length - 1 && "text-right"
            )}
          >
            {timestamp === null
              ? "—"
              : (durationSeconds >= 24 * 60 * 60
                  ? dateTimeFormatter
                  : timeFormatter
                ).format(timestamp * 1000)}
          </span>
        )
      })}
    </div>
  )
}

export function HomeMetricsCard() {
  const [preset, setPreset] = useState<TimePreset>(defaultTimePreset)
  const [cpu, setCpu] = useState(emptySnapshot)
  const [memory, setMemory] = useState(emptySnapshot)
  const [bandPreview, setBandPreview] = useState(emptySnapshot)
  const [rulerTimestamp, setRulerTimestamp] = useState<number | null>(null)
  const loadCpu = useMemo(
    () => metricLoader("cpu", preset.stepSeconds),
    [preset.stepSeconds]
  )
  const loadMemory = useMemo(
    () => metricLoader("memory", preset.stepSeconds),
    [preset.stepSeconds]
  )
  const loadBandPreview = useMemo(
    () => bandPreviewLoader(preset.stepSeconds),
    [preset.stepSeconds]
  )
  const handleCpuChange = useCallback(
    (snapshot: LiveHorizonSnapshot) => setCpu(snapshot),
    []
  )
  const handleMemoryChange = useCallback(
    (snapshot: LiveHorizonSnapshot) => setMemory(snapshot),
    []
  )
  const status = combinedStatus([cpu, memory])
  const range = cpu.range ?? memory.range
  const handlePresetChange = useCallback((values: string[]) => {
    const next = timePresets.find(({ id }) => id === values[0])
    if (!next) {
      return
    }

    setPreset(next)
    setCpu(emptySnapshot)
    setMemory(emptySnapshot)
    setBandPreview(emptySnapshot)
    setRulerTimestamp(null)
  }, [])

  return (
    <Card className="border-border/70 shadow-xs">
      <CardHeader>
        <CardTitle>IAD2 live metrics</CardTitle>
        <CardDescription>Node utilization from Prometheus.</CardDescription>
        <CardAction className="flex items-center gap-2">
          <Badge variant={badgeVariant(status)}>{statusLabel(status)}</Badge>
          <ToggleGroup
            aria-label="Metric time range"
            value={[preset.id]}
            onValueChange={handlePresetChange}
            variant="outline"
            size="sm"
            spacing={0}
          >
            {timePresets.map(({ id }) => (
              <ToggleGroupItem key={id} value={id} aria-label={`Last ${id}`}>
                {id}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </CardAction>
      </CardHeader>
      <CardContent className="flex flex-col gap-1">
        <TimeAxis range={range} durationSeconds={preset.durationSeconds} />
        <div className="flex flex-col">
          <MetricLane
            label="CPU utilization"
            load={loadCpu}
            snapshot={cpu}
            onStateChange={handleCpuChange}
            rulerTimestamp={rulerTimestamp}
            onRulerTimestampChange={setRulerTimestamp}
            preset={preset}
          />
          <MetricLane
            label="Memory utilization"
            load={loadMemory}
            snapshot={memory}
            onStateChange={handleMemoryChange}
            rulerTimestamp={rulerTimestamp}
            onRulerTimestampChange={setRulerTimestamp}
            preset={preset}
          />
          <MetricLane
            label="Color bands preview"
            load={loadBandPreview}
            snapshot={bandPreview}
            onStateChange={setBandPreview}
            rulerTimestamp={rulerTimestamp}
            onRulerTimestampChange={setRulerTimestamp}
            preset={preset}
            extent={previewExtent}
          />
        </div>
      </CardContent>
      <CardFooter className="text-xs text-muted-foreground">
        Point to a lane to inspect its value.
      </CardFooter>
    </Card>
  )
}
