import "server-only"

import { readFileSync } from "node:fs"
import { parse } from "yaml"

import { parseMetricsConfig } from "@/lib/metrics-config"

export function readMetricsConfig() {
  const path = process.env.INTERN_METRICS_CONFIG_PATH?.trim()
  if (!path) {
    return null
  }

  try {
    return parseMetricsConfig(parse(readFileSync(path, "utf8")))
  } catch {
    return null
  }
}
