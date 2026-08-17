"use client"

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react"

import {
  clampedLabelPosition,
  LiveHorizon,
  overlappingTickIndexes,
  type TimeRange,
  type TimeSeriesPoint,
  timeRangeFraction,
  timeRangeTimestampAtFraction,
  timeRangeTicks,
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
  type MetricFormat,
} from "@/lib/metrics-config"
import {
  defaultMetricTimePreset,
  type MetricTimePreset,
  type MetricTimePresetId,
  metricTimePresets,
  metricTimeRangePath,
} from "@/lib/metric-time-range"
import { useMetricsSnapshot } from "@/hooks/use-metrics-snapshot"
import {
  metricsSnapshotSeriesKey,
  type MetricsDashboardSnapshot,
  type MetricsSeriesSnapshot,
  type MetricsSeriesStatus,
} from "@/lib/metrics-snapshot-client"
import { cn } from "@/lib/utils"

type TimePreset = MetricTimePreset
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
const hourFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "numeric",
})
const secondsFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "numeric",
  minute: "2-digit",
  second: "2-digit",
})
const dateFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
})
const dateTimeFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
})

function formatStaticTimestamp(timestamp: number) {
  const date = new Date(timestamp * 1000)
  if (date.getHours() === 0 && date.getMinutes() === 0) {
    return dateFormatter.format(date)
  }

  return (date.getMinutes() === 0 ? hourFormatter : timeFormatter).format(date)
}

function formatFocusTimestamp(
  timestamp: number,
  durationSeconds: number,
  stepSeconds: number
) {
  if (durationSeconds >= 24 * 60 * 60) {
    return dateTimeFormatter.format(timestamp * 1000)
  }
  return (stepSeconds < 60 ? secondsFormatter : timeFormatter).format(
    timestamp * 1000
  )
}

const emptySnapshot: MetricsSeriesSnapshot = {
  status: "loading",
  points: [],
}

function seriesKey(groupId: string, laneId: string, seriesId: string) {
  return metricsSnapshotSeriesKey({
    group: groupId,
    lane: laneId,
    series: seriesId,
  })
}

function badgeVariant(status: MetricsSeriesStatus) {
  if (status === "live") {
    return "outline" as const
  }
  if (status === "loading") {
    return "secondary" as const
  }
  return "destructive" as const
}

const SERIES_STATUS_LABELS = {
  loading: "Loading",
  live: "Live",
  stale: "Stale",
  unavailable: "Unavailable",
} as const

function statusLabel(status: MetricsSeriesStatus) {
  return SERIES_STATUS_LABELS[status]
}

function combinedStatus(snapshots: readonly MetricsSeriesSnapshot[]) {
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
  points,
  range,
  extent,
  height,
  colors,
  ariaLabel,
  className,
  rulerTimestamp,
  onRulerTimestampChange,
  preset,
}: {
  metricKey: string
  points: readonly TimeSeriesPoint[]
  range: TimeRange | null
  extent: readonly [minimum: number, maximum: number]
  height: number
  colors: readonly string[]
  ariaLabel: string
  className?: string
  rulerTimestamp: number | null
  onRulerTimestampChange: (timestamp: number | null) => void
  preset: TimePreset
}) {
  return (
    <LiveHorizon
      key={`${metricKey}-${preset.id}`}
      points={points}
      range={range}
      stepSeconds={preset.stepSeconds}
      ariaLabel={ariaLabel}
      className={className}
      rulerTimestamp={rulerTimestamp}
      onRulerTimestampChange={onRulerTimestampChange}
      interactive={false}
      positiveColors={colors}
      extent={extent}
      height={height}
    />
  )
}

function pointFor(
  snapshot: MetricsSeriesSnapshot | undefined,
  range: TimeRange | null
) {
  if (!snapshot || !range) {
    return null
  }

  for (let index = snapshot.points.length - 1; index >= 0; index -= 1) {
    const point = snapshot.points[index]
    if (point[0] >= range.start && point[0] <= range.end) {
      return point
    }
  }
  return null
}

type SeriesReadout = { kind: "value" | "state"; text: string }

function metricReadout(
  snapshot: MetricsSeriesSnapshot | undefined,
  format: MetricFormat,
  focused: boolean,
  range: TimeRange | null,
  rulerTimestamp: number | null
): SeriesReadout {
  const point = focused
    ? (snapshot?.points.find(([timestamp]) => timestamp === rulerTimestamp) ??
      null)
    : pointFor(snapshot, range)
  if (point) {
    return { kind: "value", text: formatMetricValue(point[1], format) }
  }
  if (!snapshot || snapshot.status === "loading") {
    return { kind: "state", text: SERIES_STATUS_LABELS.loading }
  }
  return {
    kind: "state",
    text:
      snapshot.status === "unavailable"
        ? SERIES_STATUS_LABELS.unavailable
        : "No data",
  }
}

function formatReadout(readout: SeriesReadout, seriesLabel: string | null) {
  return seriesLabel !== null && readout.kind === "value"
    ? `${seriesLabel} ${readout.text}`
    : readout.text
}

function MetricLane({
  group,
  lane,
  snapshot,
  range,
  rulerTimestamp,
  onRulerTimestampChange,
  preset,
}: {
  group: ClientMetricGroup
  lane: ClientMetricLane
  snapshot: MetricsDashboardSnapshot | null
  range: TimeRange | null
  rulerTimestamp: number | null
  onRulerTimestampChange: (timestamp: number | null) => void
  preset: TimePreset
}) {
  const focusFraction = timeRangeFraction(range, rulerTimestamp)
  const focused = focusFraction !== null
  const configuredSeries = lane.series.map((series) => ({
    series,
    key: seriesKey(group.id, lane.id, series.id),
  }))
  const readouts = configuredSeries.map(({ series, key }) => ({
    series,
    readout: metricReadout(
      snapshot?.series[key] ?? emptySnapshot,
      lane.format,
      focused,
      range,
      rulerTimestamp
    ),
  }))
  const readoutStyle =
    focusFraction === null
      ? undefined
      : { right: `calc(${(1 - focusFraction) * 100}% + 0.5rem)` }
  const sharedProps = {
    extent: lane.extent,
    rulerTimestamp,
    onRulerTimestampChange,
    preset,
  }

  return (
    <div className="relative border-y border-border/60">
      {configuredSeries.length === 1 ? (
        <MetricSeriesChart
          {...sharedProps}
          metricKey={configuredSeries[0].key}
          points={snapshot?.series[configuredSeries[0].key]?.points ?? []}
          range={range}
          height={30}
          colors={positiveColors}
          ariaLabel={`${group.title}: ${lane.label}.`}
          className="bg-muted/20"
        />
      ) : (
        <div className="flex flex-col">
          <MetricSeriesChart
            {...sharedProps}
            metricKey={configuredSeries[0].key}
            points={snapshot?.series[configuredSeries[0].key]?.points ?? []}
            range={range}
            height={30}
            colors={positiveColors}
            ariaLabel={`${group.title}: ${lane.label} ${configuredSeries[0].series.label}.`}
            className="bg-muted/20"
          />
          <div>
            <div className="-scale-y-100">
              <MetricSeriesChart
                {...sharedProps}
                metricKey={configuredSeries[1].key}
                points={snapshot?.series[configuredSeries[1].key]?.points ?? []}
                range={range}
                height={30}
                colors={negativeColors}
                ariaLabel={`${group.title}: ${lane.label} ${configuredSeries[1].series.label}.`}
                className="bg-muted/20"
              />
            </div>
          </div>
        </div>
      )}
      <div className="pointer-events-none absolute inset-0 text-xs text-muted-foreground">
        <span className="horizon-label absolute top-1/2 left-2 -translate-y-1/2 whitespace-nowrap">
          {lane.label}
        </span>
        {readouts.map(({ series, readout }) => (
          <output
            key={series.id}
            className={cn(
              "horizon-label absolute right-2 -translate-y-1/2 tabular-nums",
              readouts.length === 1
                ? "top-1/2"
                : series.side === "bottom"
                  ? "top-3/4"
                  : "top-1/4"
            )}
            style={readoutStyle}
          >
            {formatReadout(
              readout,
              readouts.length === 1 ? null : series.label
            )}
          </output>
        ))}
      </div>
    </div>
  )
}

function TimeAxisTick({
  position,
  width,
  hidden,
  children,
}: {
  position: number
  width: number
  hidden: boolean
  children: string
}) {
  const [labelWidth, setLabelWidth] = useState(0)
  const measureLabel = useCallback((element: HTMLSpanElement | null) => {
    if (element) {
      setLabelWidth(element.getBoundingClientRect().width)
    }
  }, [])
  const left = clampedLabelPosition(position, width, labelWidth)

  return (
    <span
      ref={measureLabel}
      className={cn(
        "absolute top-0 -translate-x-1/2 text-center whitespace-nowrap transition-opacity duration-[250ms] ease-linear",
        hidden && "opacity-0"
      )}
      style={{ left }}
    >
      {children}
    </span>
  )
}

function TimeAxis({
  range,
  durationSeconds,
  stepSeconds,
  rulerTimestamp,
}: {
  range: TimeRange | null
  durationSeconds: number
  stepSeconds: number
  rulerTimestamp: number | null
}) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)
  const [focusLabelWidth, setFocusLabelWidth] = useState(0)
  const focusFraction = timeRangeFraction(range, rulerTimestamp)
  const ticks = useMemo(() => timeRangeTicks(range, width), [range, width])
  const tickPositions = useMemo(
    () =>
      ticks.map(
        (timestamp) => (timeRangeFraction(range, timestamp) ?? 0) * width
      ),
    [range, ticks, width]
  )
  const focusLabel =
    rulerTimestamp === null
      ? null
      : formatFocusTimestamp(rulerTimestamp, durationSeconds, stepSeconds)
  const measureFocusLabel = useCallback(
    (element: HTMLSpanElement | null) => {
      if (element && focusLabel) {
        setFocusLabelWidth(element.getBoundingClientRect().width)
      }
    },
    [focusLabel]
  )
  const focusPosition =
    focusFraction === null
      ? null
      : clampedLabelPosition(focusFraction * width, width, focusLabelWidth)
  const overlappingTicks = useMemo(
    () =>
      new Set(
        overlappingTickIndexes(tickPositions, focusPosition, focusLabelWidth)
      ),
    [focusLabelWidth, focusPosition, tickPositions]
  )

  useEffect(() => {
    const element = containerRef.current
    if (!element) {
      return
    }

    const measure = () => setWidth(Math.max(1, Math.floor(element.clientWidth)))
    measure()

    const observer = new ResizeObserver(measure)
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  return (
    <div
      ref={containerRef}
      className="relative h-4 text-[11px] text-muted-foreground tabular-nums"
    >
      {ticks.map((timestamp, index) => (
        <TimeAxisTick
          key={timestamp}
          position={tickPositions[index]}
          width={width}
          hidden={overlappingTicks.has(index)}
        >
          {formatStaticTimestamp(timestamp)}
        </TimeAxisTick>
      ))}
      {focusPosition !== null && focusLabel !== null ? (
        <span
          ref={measureFocusLabel}
          className="pointer-events-none absolute top-0 -translate-x-1/2 rounded-sm bg-card px-1 text-center whitespace-nowrap text-foreground"
          style={{ left: focusPosition }}
        >
          {focusLabel}
        </span>
      ) : null}
    </div>
  )
}

function MetricsGroupCard({
  group,
  snapshot,
  range,
  rulerTimestamp,
  onRulerTimestampChange,
  preset,
}: {
  group: ClientMetricGroup
  snapshot: MetricsDashboardSnapshot | null
  range: TimeRange | null
  rulerTimestamp: number | null
  onRulerTimestampChange: (timestamp: number | null) => void
  preset: TimePreset
}) {
  const groupSnapshots = group.lanes.flatMap((lane) =>
    lane.series.map(
      (series) =>
        snapshot?.series[seriesKey(group.id, lane.id, series.id)] ??
        emptySnapshot
    )
  )
  const status = combinedStatus(groupSnapshots)
  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const bounds = event.currentTarget.getBoundingClientRect()
    const timestamp = timeRangeTimestampAtFraction(
      range,
      (event.clientX - bounds.left) / Math.max(1, bounds.width),
      preset.stepSeconds
    )
    if (timestamp !== null) {
      onRulerTimestampChange(timestamp)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{group.title}</CardTitle>
        {group.subtitle ? (
          <CardDescription>{group.subtitle}</CardDescription>
        ) : null}
        <CardAction>
          <Badge className="h-6 px-2.5 py-1" variant={badgeVariant(status)}>
            {status === "live" ? (
              <span
                className="size-1.5 shrink-0 animate-pulse rounded-full bg-destructive motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : null}
            {statusLabel(status)}
          </Badge>
        </CardAction>
      </CardHeader>
      <CardContent className="flex flex-col gap-1">
        <TimeAxis
          range={range}
          durationSeconds={preset.durationSeconds}
          stepSeconds={preset.stepSeconds}
          rulerTimestamp={rulerTimestamp}
        />
        <div
          className="flex touch-none flex-col"
          onPointerMove={handlePointerMove}
          onPointerLeave={() => onRulerTimestampChange(null)}
        >
          {group.lanes.map((lane) => (
            <MetricLane
              key={lane.id}
              group={group}
              lane={lane}
              snapshot={snapshot}
              range={range}
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
  initialPresetId,
}: {
  groups: readonly ClientMetricGroup[] | null
  initialPresetId: MetricTimePresetId
}) {
  const [preset, setPreset] = useState<TimePreset>(
    () =>
      metricTimePresets.find(({ id }) => id === initialPresetId) ??
      defaultMetricTimePreset
  )
  const snapshot = useMetricsSnapshot(groups, preset)
  const range = snapshot?.range ?? null
  const [rulerTimestamp, setRulerTimestamp] = useState<number | null>(null)
  const replaceRangeQuery = useCallback((presetId: MetricTimePresetId) => {
    window.history.replaceState(
      null,
      "",
      metricTimeRangePath(window.location.href, presetId)
    )
  }, [])
  const handlePresetChange = useCallback(
    (values: string[]) => {
      const next = metricTimePresets.find(({ id }) => id === values[0])
      if (!next) {
        return
      }

      setPreset(next)
      setRulerTimestamp(null)
      replaceRangeQuery(next.id)
    },
    [replaceRangeQuery]
  )

  useEffect(() => {
    replaceRangeQuery(initialPresetId)
  }, [initialPresetId, replaceRangeQuery])

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
          {metricTimePresets.map(({ id }) => (
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
          snapshot={snapshot}
          range={range}
          rulerTimestamp={rulerTimestamp}
          onRulerTimestampChange={setRulerTimestamp}
          preset={preset}
        />
      ))}
    </div>
  )
}
