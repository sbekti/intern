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
  timeRangeFraction,
  timeRangeTimestampAtFraction,
  timeSeriesExtent,
  type TimeRange,
  type TimeSeriesPoint,
} from "./model"
import { HorizonRenderer, type HorizonMode } from "./renderer"

export type LiveHorizonProps = {
  points: readonly TimeSeriesPoint[]
  range: TimeRange | null
  stepSeconds: number
  ariaLabel: string
  className?: string
  rulerTimestamp?: number | null
  onRulerTimestampChange?: (timestamp: number | null) => void
  interactive?: boolean
  height?: number
  bands?: number
  extent?: readonly [minimum: number, maximum: number]
  extentHeadroom?: number
  positiveColors?: readonly string[]
  negativeColors?: readonly string[]
  mode?: HorizonMode
  overlapSteps?: number
}

const defaultPositiveColors = [
  "var(--horizon-positive-1)",
  "var(--horizon-positive-2)",
  "var(--horizon-positive-3)",
  "var(--horizon-positive-4)",
]
const defaultNegativeColors = [
  "var(--horizon-negative-1)",
  "var(--horizon-negative-2)",
  "var(--horizon-negative-3)",
  "var(--horizon-negative-4)",
]
function resolveColor(element: HTMLElement, color: string) {
  const match = color.match(/^var\((--[^)]+)\)$/)
  return match
    ? getComputedStyle(element).getPropertyValue(match[1]).trim()
    : color
}

export function LiveHorizon({
  points,
  range,
  stepSeconds,
  ariaLabel,
  className,
  rulerTimestamp: controlledRulerTimestamp,
  onRulerTimestampChange,
  interactive = true,
  height = 120,
  bands = 4,
  extent,
  extentHeadroom = 0,
  positiveColors = defaultPositiveColors,
  negativeColors = defaultNegativeColors,
  mode = "offset",
  overlapSteps = 6,
}: LiveHorizonProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const rendererRef = useRef<HorizonRenderer | null>(null)
  const [width, setWidth] = useState(0)
  const [internalRulerTimestamp, setInternalRulerTimestamp] = useState<
    number | null
  >(null)
  const [themeRevision, setThemeRevision] = useState(0)
  const measured = width > 0
  const columnCount = range
    ? Math.floor((range.end - range.start) / stepSeconds) + 1
    : width

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
      rendererRef.current?.reset()
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

  const moveRuler = useCallback(
    (clientX: number) => {
      const element = containerRef.current
      if (!element || !range) {
        return
      }

      const bounds = element.getBoundingClientRect()
      const timestamp = timeRangeTimestampAtFraction(
        range,
        (clientX - bounds.left) / Math.max(1, bounds.width),
        stepSeconds
      )
      if (timestamp !== null) {
        setRulerTimestamp(timestamp)
      }
    },
    [range, setRulerTimestamp, stepSeconds]
  )

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    moveRuler(event.clientX)
  }

  const rulerFraction = timeRangeFraction(range, rulerTimestamp)
  const rulerLeft = rulerFraction === null ? null : `${rulerFraction * 100}%`

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
      onPointerMove={interactive ? handlePointerMove : undefined}
      onPointerLeave={interactive ? () => setRulerTimestamp(null) : undefined}
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
