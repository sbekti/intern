"use client"

import { useCallback, useMemo, useState } from "react"

import {
  isTimeSeriesPayload,
  LiveHorizon,
  type LiveHorizonSnapshot,
  type LiveHorizonStatus,
  type TimeRange,
  type TimeSeriesLoader,
} from "@/components/live-horizon"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import {
  formatMetricValue,
  type ClientMetricGroup,
  type ClientMetricLane,
  type ClientMetricSeries,
} from "@/lib/metrics-config"
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
const negativeColors = [
  "var(--horizon-negative-1)",
  "var(--horizon-negative-2)",
  "var(--horizon-negative-3)",
  "var(--horizon-negative-4)",
] as const
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

function seriesKey(groupId: string, laneId: string, seriesId: string) {
  return `${groupId}/${laneId}/${seriesId}`
}

function metricLoader(
  groupId: string,
  laneId: string,
  series: ClientMetricSeries,
  stepSeconds: number
): TimeSeriesLoader {
  return async (range, signal) => {
    const query = new URLSearchParams({
      group: groupId,
      lane: laneId,
      series: series.id,
      step: String(stepSeconds),
      start: String(range.start),
      end: String(range.end),
    })
    const response = await fetch(`/bff/metrics?${query}`, {
      cache: "no-store",
      signal,
    })

    if (!response.ok) {
      throw new Error(`${series.label} metric request failed`)
    }

    const body: unknown = await response.json()
    if (!isTimeSeriesPayload(body, stepSeconds)) {
      throw new Error(`${series.label} metric response was invalid`)
    }

    return body.points
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

function MetricSeriesChart({
  metricKey,
  load,
  extent,
  height,
  colors,
  ariaLabel,
  className,
  rulerTimestamp,
  onRulerTimestampChange,
  onSnapshotChange,
  preset,
}: {
  metricKey: string
  load: TimeSeriesLoader
  extent: readonly [minimum: number, maximum: number]
  height: number
  colors: readonly string[]
  ariaLabel: string
  className?: string
  rulerTimestamp: number | null
  onRulerTimestampChange: (timestamp: number | null) => void
  onSnapshotChange: (key: string, snapshot: LiveHorizonSnapshot) => void
  preset: TimePreset
}) {
  const handleStateChange = useCallback(
    (snapshot: LiveHorizonSnapshot) => onSnapshotChange(metricKey, snapshot),
    [metricKey, onSnapshotChange]
  )

  return (
    <LiveHorizon
      key={`${metricKey}-${preset.id}`}
      load={load}
      stepSeconds={preset.stepSeconds}
      windowSeconds={preset.durationSeconds}
      ariaLabel={ariaLabel}
      className={className}
      rulerTimestamp={rulerTimestamp}
      onRulerTimestampChange={onRulerTimestampChange}
      onStateChange={handleStateChange}
      positiveColors={colors}
      extent={extent}
      height={height}
    />
  )
}

function pointFor(snapshot: LiveHorizonSnapshot | undefined) {
  return snapshot?.rulerPoint ?? snapshot?.latestPoint ?? null
}

function MetricLane({
  group,
  lane,
  loaders,
  snapshots,
  onSnapshotChange,
  rulerTimestamp,
  onRulerTimestampChange,
  preset,
}: {
  group: ClientMetricGroup
  lane: ClientMetricLane
  loaders: Readonly<Record<string, TimeSeriesLoader>>
  snapshots: Readonly<Record<string, LiveHorizonSnapshot>>
  onSnapshotChange: (key: string, snapshot: LiveHorizonSnapshot) => void
  rulerTimestamp: number | null
  onRulerTimestampChange: (timestamp: number | null) => void
  preset: TimePreset
}) {
  const configuredSeries = lane.series.map((series) => ({
    series,
    key: seriesKey(group.id, lane.id, series.id),
  }))
  const readout = configuredSeries
    .map(({ series, key }) => {
      const point = pointFor(snapshots[key])
      const value = point ? formatMetricValue(point[1], lane.format) : "—"
      return lane.series.length === 1 ? value : `${series.label} ${value}`
    })
    .join(" · ")
  const sharedProps = {
    extent: lane.extent,
    rulerTimestamp,
    onRulerTimestampChange,
    onSnapshotChange,
    preset,
  }

  return (
    <div className="relative border-y border-border/60">
      {configuredSeries.length === 1 ? (
        <MetricSeriesChart
          {...sharedProps}
          metricKey={configuredSeries[0].key}
          load={loaders[configuredSeries[0].key]}
          height={64}
          colors={positiveColors}
          ariaLabel={`${group.title}: ${lane.label}.`}
          className="bg-muted/20"
        />
      ) : (
        <div className="flex flex-col">
          <MetricSeriesChart
            {...sharedProps}
            metricKey={configuredSeries[0].key}
            load={loaders[configuredSeries[0].key]}
            height={32}
            colors={positiveColors}
            ariaLabel={`${group.title}: ${lane.label} ${configuredSeries[0].series.label}.`}
            className="bg-muted/20"
          />
          <div className="border-t border-border/60">
            <div className="-scale-y-100">
              <MetricSeriesChart
                {...sharedProps}
                metricKey={configuredSeries[1].key}
                load={loaders[configuredSeries[1].key]}
                height={32}
                colors={negativeColors}
                ariaLabel={`${group.title}: ${lane.label} ${configuredSeries[1].series.label}.`}
                className="bg-muted/20"
              />
            </div>
          </div>
        </div>
      )}
      <div className="pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between gap-4 p-2 text-sm">
        <span className="rounded-sm bg-background/80 px-1.5 py-0.5 font-medium backdrop-blur-xs">
          {lane.label}
        </span>
        <output className="rounded-sm bg-background/80 px-1.5 py-0.5 font-semibold tabular-nums backdrop-blur-xs">
          {readout}
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

function MetricsGroupCard({
  group,
  loaders,
  snapshots,
  onSnapshotChange,
  rulerTimestamp,
  onRulerTimestampChange,
  preset,
}: {
  group: ClientMetricGroup
  loaders: Readonly<Record<string, TimeSeriesLoader>>
  snapshots: Readonly<Record<string, LiveHorizonSnapshot>>
  onSnapshotChange: (key: string, snapshot: LiveHorizonSnapshot) => void
  rulerTimestamp: number | null
  onRulerTimestampChange: (timestamp: number | null) => void
  preset: TimePreset
}) {
  const groupSnapshots = group.lanes.flatMap((lane) =>
    lane.series.map(
      (series) =>
        snapshots[seriesKey(group.id, lane.id, series.id)] ?? emptySnapshot
    )
  )
  const status = combinedStatus(groupSnapshots)
  const range = groupSnapshots.find((snapshot) => snapshot.range)?.range ?? null

  return (
    <Card>
      <CardHeader>
        <CardTitle>{group.title}</CardTitle>
        {group.subtitle ? (
          <CardDescription>{group.subtitle}</CardDescription>
        ) : null}
        <CardAction>
          <Badge variant={badgeVariant(status)}>{statusLabel(status)}</Badge>
        </CardAction>
      </CardHeader>
      <CardContent className="flex flex-col gap-1">
        <TimeAxis range={range} durationSeconds={preset.durationSeconds} />
        <div className="flex flex-col">
          {group.lanes.map((lane) => (
            <MetricLane
              key={lane.id}
              group={group}
              lane={lane}
              loaders={loaders}
              snapshots={snapshots}
              onSnapshotChange={onSnapshotChange}
              rulerTimestamp={rulerTimestamp}
              onRulerTimestampChange={onRulerTimestampChange}
              preset={preset}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

export function HomeMetricsDashboard({
  groups,
}: {
  groups: readonly ClientMetricGroup[] | null
}) {
  const [preset, setPreset] = useState<TimePreset>(defaultTimePreset)
  const [snapshots, setSnapshots] = useState<
    Record<string, LiveHorizonSnapshot>
  >({})
  const [rulerTimestamp, setRulerTimestamp] = useState<number | null>(null)
  const loaders = useMemo(() => {
    if (!groups) {
      return {}
    }

    return Object.fromEntries(
      groups.flatMap((group) =>
        group.lanes.flatMap((lane) =>
          lane.series.map((series) => {
            const key = seriesKey(group.id, lane.id, series.id)
            return [
              key,
              metricLoader(group.id, lane.id, series, preset.stepSeconds),
            ]
          })
        )
      )
    )
  }, [groups, preset.stepSeconds])
  const handleSnapshotChange = useCallback(
    (key: string, snapshot: LiveHorizonSnapshot) => {
      setSnapshots((current) => ({ ...current, [key]: snapshot }))
    },
    []
  )
  const handlePresetChange = useCallback((values: string[]) => {
    const next = timePresets.find(({ id }) => id === values[0])
    if (!next) {
      return
    }

    setPreset(next)
    setSnapshots({})
    setRulerTimestamp(null)
  }, [])

  if (!groups) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Metrics unavailable</CardTitle>
          <CardDescription>
            Check the frontend metrics configuration.
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
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
      </div>
      {groups.map((group) => (
        <MetricsGroupCard
          key={group.id}
          group={group}
          loaders={loaders}
          snapshots={snapshots}
          onSnapshotChange={handleSnapshotChange}
          rulerTimestamp={rulerTimestamp}
          onRulerTimestampChange={setRulerTimestamp}
          preset={preset}
        />
      ))}
    </div>
  )
}
