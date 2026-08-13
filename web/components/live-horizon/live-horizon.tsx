"use client"

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent,
} from "react"

import {
  mergeTimeSeriesPoints,
  timeSeriesExtent,
  type TimeSeriesLoader,
  type TimeSeriesPoint,
} from "./model"
import { HorizonRenderer, type HorizonMode } from "./renderer"

export type LiveHorizonStatus = "loading" | "live" | "stale" | "unavailable"

export type LiveHorizonSnapshot = {
  status: LiveHorizonStatus
  latestPoint: TimeSeriesPoint | null
  rulerPoint: TimeSeriesPoint | null
  range: { start: number; end: number } | null
}

export type LiveHorizonProps = {
  load: TimeSeriesLoader
  stepSeconds: number
  windowSeconds?: number
  ariaLabel: string
  className?: string
  rulerTimestamp?: number | null
  onRulerTimestampChange?: (timestamp: number | null) => void
  onStateChange?: (snapshot: LiveHorizonSnapshot) => void
  height?: number
  bands?: number
  extent?: readonly [minimum: number, maximum: number]
  extentHeadroom?: number
  positiveColors?: readonly string[]
  negativeColors?: readonly string[]
  mode?: HorizonMode
  overlapSteps?: number
  maxPoints?: number
  serverDelaySeconds?: number
}

const defaultPositiveColors = ["#d1fae5", "#6ee7b7", "#10b981", "#047857"]
const defaultNegativeColors = ["#fee2e2", "#fca5a5", "#ef4444", "#b91c1c"]
function alignedEnd(stepSeconds: number, serverDelaySeconds: number) {
  return (
    Math.floor((Date.now() / 1000 - serverDelaySeconds) / stepSeconds) *
    stepSeconds
  )
}

function nextRefreshDelay(stepSeconds: number, serverDelaySeconds: number) {
  const stepMilliseconds = stepSeconds * 1000
  const delayedNow = Date.now() - serverDelaySeconds * 1000
  return stepMilliseconds - (delayedNow % stepMilliseconds) + 250
}

function resolveColor(element: HTMLElement, color: string) {
  const match = color.match(/^var\((--[^)]+)\)$/)
  return match
    ? getComputedStyle(element).getPropertyValue(match[1]).trim()
    : color
}

function pointInRange(
  points: readonly TimeSeriesPoint[],
  start: number,
  end: number
) {
  for (let index = points.length - 1; index >= 0; index -= 1) {
    const point = points[index]
    if (point[0] <= end && point[0] >= start) {
      return point
    }
  }
  return null
}

export function LiveHorizon({
  load,
  stepSeconds,
  windowSeconds,
  ariaLabel,
  className,
  rulerTimestamp: controlledRulerTimestamp,
  onRulerTimestampChange,
  onStateChange,
  height = 120,
  bands = 4,
  extent,
  extentHeadroom = 0,
  positiveColors = defaultPositiveColors,
  negativeColors = defaultNegativeColors,
  mode = "offset",
  overlapSteps = 6,
  maxPoints = 4096,
  serverDelaySeconds = 5,
}: LiveHorizonProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const rendererRef = useRef<HorizonRenderer | null>(null)
  const pointsRef = useRef<readonly TimeSeriesPoint[]>([])
  const [width, setWidth] = useState(0)
  const [points, setPoints] = useState<readonly TimeSeriesPoint[]>([])
  const [range, setRange] = useState<{ start: number; end: number } | null>(
    null
  )
  const [status, setStatus] = useState<LiveHorizonStatus>("loading")
  const [internalRulerTimestamp, setInternalRulerTimestamp] = useState<
    number | null
  >(null)
  const [themeRevision, setThemeRevision] = useState(0)
  const measured = width > 0
  const columnCount =
    windowSeconds === undefined
      ? width
      : Math.floor(windowSeconds / stepSeconds) + 1

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

  useEffect(() => {
    const observer = new MutationObserver(() => {
      rendererRef.current?.reset()
      setThemeRevision((revision) => revision + 1)
    })
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    })
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    if (!measured) {
      return
    }

    let active = true
    let timer: ReturnType<typeof setTimeout> | undefined
    let controller: AbortController | undefined

    const refresh = async () => {
      const end = alignedEnd(stepSeconds, serverDelaySeconds)
      const start = end - (columnCount - 1) * stepSeconds
      const current = pointsRef.current
      const earliest = current[0]?.[0]
      const latest = current[current.length - 1]?.[0]
      const requestStart =
        earliest === undefined || earliest > start || latest === undefined
          ? start
          : Math.max(start, latest - overlapSteps * stepSeconds)

      controller = new AbortController()
      try {
        const incoming = await load(
          { start: requestStart, end },
          controller.signal
        )
        if (!active) {
          return
        }
        if (incoming.length === 0 && current.length === 0) {
          throw new Error("The time-series loader returned no points")
        }

        const retentionStart = end - (maxPoints - 1) * stepSeconds
        const merged = mergeTimeSeriesPoints(current, incoming, retentionStart)
        pointsRef.current = merged
        setPoints(merged)
        setRange({ start, end })
        setStatus("live")
      } catch (error) {
        if (
          !active ||
          (error instanceof DOMException && error.name === "AbortError")
        ) {
          return
        }
        setStatus(pointsRef.current.length > 0 ? "stale" : "unavailable")
      } finally {
        if (active) {
          timer = setTimeout(
            refresh,
            nextRefreshDelay(stepSeconds, serverDelaySeconds)
          )
        }
      }
    }

    void refresh()
    return () => {
      active = false
      controller?.abort()
      if (timer) {
        clearTimeout(timer)
      }
    }
  }, [
    columnCount,
    load,
    maxPoints,
    measured,
    overlapSteps,
    serverDelaySeconds,
    stepSeconds,
  ])

  const visiblePoints = useMemo(
    () =>
      range
        ? points.filter(
            ([timestamp]) => timestamp >= range.start && timestamp <= range.end
          )
        : [],
    [points, range]
  )
  const effectiveExtent = useMemo(
    () => extent ?? timeSeriesExtent(visiblePoints, extentHeadroom),
    [extent, extentHeadroom, visiblePoints]
  )
  const rulerTimestamp =
    controlledRulerTimestamp === undefined
      ? internalRulerTimestamp
      : controlledRulerTimestamp
  const setRulerTimestamp = useCallback(
    (timestamp: number | null) => {
      if (controlledRulerTimestamp === undefined) {
        setInternalRulerTimestamp(timestamp)
      }
      onRulerTimestampChange?.(timestamp)
    },
    [controlledRulerTimestamp, onRulerTimestampChange]
  )

  useEffect(() => {
    const canvas = canvasRef.current
    const container = containerRef.current
    if (!canvas || !container || !range || !measured) {
      return
    }

    const renderer = rendererRef.current ?? new HorizonRenderer(canvas)
    rendererRef.current = renderer
    renderer.render({
      points,
      ...range,
      stepSeconds,
      width: columnCount,
      height,
      pixelRatio: window.devicePixelRatio || 1,
      bands,
      extent: effectiveExtent,
      positiveColors: positiveColors.map((color) =>
        resolveColor(container, color)
      ),
      negativeColors: negativeColors.map((color) =>
        resolveColor(container, color)
      ),
      mode,
      overlapSteps,
    })
  }, [
    bands,
    columnCount,
    effectiveExtent,
    height,
    mode,
    negativeColors,
    overlapSteps,
    points,
    positiveColors,
    range,
    measured,
    stepSeconds,
    themeRevision,
  ])

  const rulerPoint =
    rulerTimestamp === null
      ? null
      : (visiblePoints.find(([timestamp]) => timestamp === rulerTimestamp) ??
        null)
  const latestPoint = range
    ? pointInRange(visiblePoints, range.start, range.end)
    : null

  useEffect(() => {
    onStateChange?.({ status, latestPoint, rulerPoint, range })
  }, [latestPoint, onStateChange, range, rulerPoint, status])

  const moveRuler = useCallback(
    (clientX: number) => {
      const element = containerRef.current
      if (!element || !range || visiblePoints.length === 0) {
        return
      }

      const bounds = element.getBoundingClientRect()
      const fraction = Math.min(
        1,
        Math.max(0, (clientX - bounds.left) / Math.max(1, bounds.width))
      )
      const target = range.start + fraction * (range.end - range.start)
      let nearest = visiblePoints[0]
      for (const point of visiblePoints) {
        if (Math.abs(point[0] - target) < Math.abs(nearest[0] - target)) {
          nearest = point
        }
      }
      setRulerTimestamp(nearest[0])
    },
    [range, setRulerTimestamp, visiblePoints]
  )

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    moveRuler(event.clientX)
  }

  const rulerLeft =
    range &&
    range.end > range.start &&
    rulerTimestamp !== null &&
    rulerTimestamp >= range.start &&
    rulerTimestamp <= range.end
      ? `${((rulerTimestamp - range.start) / (range.end - range.start)) * 100}%`
      : null

  return (
    <div
      ref={containerRef}
      className={className}
      style={{
        position: "relative",
        width: "100%",
        height,
        overflow: "hidden",
        touchAction: "none",
      }}
      role="img"
      aria-label={ariaLabel}
      onPointerMove={handlePointerMove}
      onPointerLeave={() => setRulerTimestamp(null)}
    >
      <canvas
        ref={canvasRef}
        aria-hidden="true"
        style={{ display: "block", width: "100%", height: "100%" }}
      />
      {rulerLeft !== null ? (
        <span
          style={{
            position: "absolute",
            top: 0,
            bottom: 0,
            left: rulerLeft,
            width: 1,
            background: "currentColor",
            opacity: 0.6,
            pointerEvents: "none",
          }}
          aria-hidden="true"
        />
      ) : null}
    </div>
  )
}
