"use client"

import { useEffect, useMemo, useRef, useState } from "react"

import type { MetricTimePreset } from "@/lib/metric-time-range"
import type { ClientMetricGroup } from "@/lib/metrics-config"
import {
  failMetricsSnapshot,
  mergeMetricsSnapshot,
  metricsSnapshotSeriesKey,
  validateMetricsSnapshot,
  type MetricsDashboardSnapshot,
  type MetricsSnapshotIdentity,
} from "@/lib/metrics-snapshot-client"

const collectionDelaySeconds = 5
const overlapSteps = 6

function documentIsHidden() {
  return document.visibilityState === "hidden"
}

type ActiveSnapshot = MetricsDashboardSnapshot & {
  presetId: MetricTimePreset["id"]
  identitySignature: string
}

function configuredIdentities(
  groups: readonly ClientMetricGroup[] | null
): MetricsSnapshotIdentity[] {
  return (
    groups?.flatMap((group) =>
      group.lanes.flatMap((lane) =>
        lane.series.map((series) => ({
          group: group.id,
          lane: lane.id,
          series: series.id,
        }))
      )
    ) ?? []
  )
}

function alignedEnd(nowMilliseconds: number, stepSeconds: number) {
  return (
    Math.floor(
      (nowMilliseconds / 1000 - collectionDelaySeconds) / stepSeconds
    ) * stepSeconds
  )
}

function nextRefreshDelay(nowMilliseconds: number, cadenceSeconds: number) {
  const cadenceMilliseconds = cadenceSeconds * 1000
  const delayedNow = nowMilliseconds - collectionDelaySeconds * 1000
  const remainder = delayedNow % cadenceMilliseconds
  return cadenceMilliseconds - remainder + 250
}

function visibleRange(
  end: number,
  preset: Pick<MetricTimePreset, "durationSeconds">
) {
  return { start: end - preset.durationSeconds, end }
}

function hasPoints(snapshot: MetricsDashboardSnapshot) {
  return Object.values(snapshot.series).some(({ points }) => points.length > 0)
}

export function useMetricsSnapshot(
  groups: readonly ClientMetricGroup[] | null,
  preset: MetricTimePreset
) {
  const identities = useMemo(() => configuredIdentities(groups), [groups])
  const identitySignature = useMemo(
    () => identities.map(metricsSnapshotSeriesKey).join("|"),
    [identities]
  )
  const [snapshot, setSnapshot] = useState<ActiveSnapshot | null>(null)
  const snapshotRef = useRef<ActiveSnapshot | null>(null)

  useEffect(() => {
    let active = true
    let timer: ReturnType<typeof setTimeout> | undefined
    let inFlight = false
    let hiddenAt: number | null = documentIsHidden() ? Date.now() : null
    let forceFullLoad = false
    let controller: AbortController | undefined
    const cadenceSeconds = Math.max(30, preset.stepSeconds)
    const cadenceMilliseconds = cadenceSeconds * 1000

    snapshotRef.current = null
    setSnapshot(null)
    if (identities.length === 0) {
      return
    }

    const isCurrent = (requestController: AbortController) =>
      active &&
      controller === requestController &&
      !requestController.signal.aborted

    const schedule = () => {
      if (!active || documentIsHidden() || inFlight || timer !== undefined) {
        return
      }

      timer = setTimeout(
        () => {
          timer = undefined
          void refresh()
        },
        nextRefreshDelay(Date.now(), cadenceSeconds)
      )
    }

    const refresh = async () => {
      if (!active || documentIsHidden() || inFlight) {
        return
      }

      inFlight = true
      const requestController = new AbortController()
      controller = requestController

      const end = alignedEnd(Date.now(), preset.stepSeconds)
      const range = visibleRange(end, preset)
      const previousEnd = snapshotRef.current?.range.end
      const start =
        forceFullLoad || previousEnd === undefined
          ? range.start
          : Math.max(
              range.start,
              previousEnd - overlapSteps * preset.stepSeconds
            )
      try {
        const params = new URLSearchParams({
          start: String(start),
          end: String(end),
          step: String(preset.stepSeconds),
        })
        const response = await fetch(`/bff/metrics/snapshot?${params}`, {
          cache: "no-store",
          signal: requestController.signal,
        })
        if (!response.ok) {
          throw new Error("Metrics snapshot request failed")
        }

        let body: unknown
        try {
          body = await response.json()
        } catch {
          throw new Error("Metrics snapshot response was invalid")
        }

        const data = validateMetricsSnapshot(
          body,
          { start, end, stepSeconds: preset.stepSeconds },
          identities
        )
        if (!data) {
          throw new Error("Metrics snapshot response was invalid")
        }
        if (!isCurrent(requestController)) {
          return
        }

        const merged = mergeMetricsSnapshot(snapshotRef.current, data, range)
        const next: ActiveSnapshot = {
          ...merged,
          presetId: preset.id,
          identitySignature,
        }
        forceFullLoad = !hasPoints(next)
        snapshotRef.current = next
        setSnapshot(next)
      } catch (error) {
        if (
          !isCurrent(requestController) ||
          (error instanceof DOMException && error.name === "AbortError") ||
          documentIsHidden()
        ) {
          return
        }

        const failed = failMetricsSnapshot(
          snapshotRef.current,
          identities,
          range,
          preset.stepSeconds
        )
        const next: ActiveSnapshot = {
          ...failed,
          presetId: preset.id,
          identitySignature,
        }
        forceFullLoad = !hasPoints(next)
        snapshotRef.current = next
        setSnapshot(next)
      } finally {
        if (controller !== requestController) {
          return
        }
        inFlight = false
        controller = undefined
        if (active && !documentIsHidden()) {
          schedule()
        }
      }
    }

    const handleVisibilityChange = () => {
      if (documentIsHidden()) {
        hiddenAt = Date.now()
        if (timer !== undefined) {
          clearTimeout(timer)
          timer = undefined
        }
        controller?.abort()
        controller = undefined
        inFlight = false
        return
      }

      if (hiddenAt !== null) {
        forceFullLoad =
          forceFullLoad || Date.now() - hiddenAt >= cadenceMilliseconds
      }
      hiddenAt = null
      if (timer !== undefined) {
        clearTimeout(timer)
        timer = undefined
      }
      void refresh()
    }

    document.addEventListener("visibilitychange", handleVisibilityChange)
    if (!documentIsHidden()) {
      void refresh()
    }

    return () => {
      active = false
      controller?.abort()
      controller = undefined
      if (timer !== undefined) {
        clearTimeout(timer)
      }
      document.removeEventListener("visibilitychange", handleVisibilityChange)
    }
  }, [identities, identitySignature, preset])

  if (
    snapshot?.presetId !== preset.id ||
    snapshot.identitySignature !== identitySignature
  ) {
    return null
  }
  return snapshot
}
