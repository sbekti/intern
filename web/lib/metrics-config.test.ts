import assert from "node:assert/strict"
import test from "node:test"

import {
  clientMetricGroups,
  findMetricSeries,
  formatMetricValue,
  parseMetricsConfig,
} from "./metrics-config.ts"

const validConfig = {
  groups: [
    {
      id: "edge",
      title: "Edge metrics",
      subtitle: "Traffic from Prometheus.",
      lanes: [
        {
          id: "wan",
          label: "WAN traffic",
          format: "bits-per-second",
          extent: [0, 300_000_000],
          series: [
            {
              id: "positive",
              label: "Positive",
              side: "top",
              promql: "positive_query",
            },
            {
              id: "negative",
              label: "Negative",
              side: "bottom",
              promql: "negative_query",
            },
          ],
        },
      ],
    },
  ],
}

test("parses groups, lanes, and split-direction series", () => {
  const config = parseMetricsConfig(validConfig)
  assert.ok(config)
  assert.equal(config.groups[0].lanes[0].extent[1], 300_000_000)
  assert.equal(
    findMetricSeries(config, "edge", "wan", "negative")?.promql,
    "negative_query"
  )
})

test("removes PromQL from client configuration", () => {
  const config = parseMetricsConfig(validConfig)
  assert.ok(config)

  const groups = clientMetricGroups(config)
  assert.deepEqual(groups[0].lanes[0].series[0], {
    id: "positive",
    label: "Positive",
    side: "top",
  })
})

test("rejects duplicate IDs and incomplete split lanes", () => {
  const duplicateGroups = structuredClone(validConfig)
  duplicateGroups.groups.push(duplicateGroups.groups[0])
  assert.equal(parseMetricsConfig(duplicateGroups), null)

  const duplicateSides = structuredClone(validConfig)
  duplicateSides.groups[0].lanes[0].series[1].side = "top"
  assert.equal(parseMetricsConfig(duplicateSides), null)
})

test("formats percent and SI traffic values", () => {
  assert.equal(formatMetricValue(52.345, "percent"), "52.3%")
  assert.equal(formatMetricValue(0, "bits-per-second"), "0 bps")
  assert.equal(formatMetricValue(800, "bits-per-second"), "800 bps")
  assert.equal(formatMetricValue(1_230_000, "bits-per-second"), "1.2 Mbps")
  assert.equal(formatMetricValue(12_300_000, "bits-per-second"), "12 Mbps")
  assert.equal(formatMetricValue(300_000_000, "bits-per-second"), "300 Mbps")
  assert.equal(formatMetricValue(999_000, "bits-per-second"), "1.0 Mbps")
  assert.equal(formatMetricValue(1_000_000_000, "bits-per-second"), "1.0 Gbps")
  assert.equal(formatMetricValue(-1_230, "bits-per-second"), "-1.2 kbps")
})
