import assert from "node:assert/strict"
import test from "node:test"

import { parseMetricsConfig } from "./metrics-config.ts"
import { getMetricsSnapshot } from "./metrics-snapshot.ts"

const config = parseMetricsConfig({
  groups: [
    {
      id: "group-a",
      title: "Group A",
      lanes: [
        {
          id: "lane-a",
          label: "Lane A",
          format: "count",
          extent: [0, 100],
          series: [{ id: "series-a", label: "Series A", promql: "expr-a" }],
        },
        {
          id: "lane-b",
          label: "Lane B",
          format: "count",
          extent: [0, 100],
          series: [
            {
              id: "series-b",
              label: "Series B",
              side: "top",
              promql: "expr-b",
            },
            {
              id: "series-c",
              label: "Series C",
              side: "bottom",
              promql: "expr-c",
            },
          ],
        },
      ],
    },
    {
      id: "group-b",
      title: "Group B",
      lanes: [
        {
          id: "lane-c",
          label: "Lane C",
          format: "count",
          extent: [0, 100],
          series: [{ id: "series-d", label: "Series D", promql: "expr-d" }],
        },
      ],
    },
  ],
})

assert.ok(config)

const baseUrl = new URL("https://metrics.test/")

function requestUrl(input: string | URL | Request) {
  return new URL(input instanceof Request ? input.url : input)
}

function matrixResponse(
  points: readonly (readonly [number, string])[] = [
    [100, "1"],
    [130, "2"],
  ]
) {
  return new Response(
    JSON.stringify({
      status: "success",
      data: {
        resultType: "matrix",
        result: [{ values: points }],
      },
    }),
    { headers: { "content-type": "application/json" } }
  )
}

function queryParams() {
  return new URLSearchParams({ start: "100", end: "160", step: "60" })
}

test("queries every configured series concurrently and preserves config order", async () => {
  const requests: { url: URL; init: RequestInit | undefined }[] = []
  let active = 0
  let maximumActive = 0

  const result = await getMetricsSnapshot(queryParams(), config, baseUrl, {
    fetcher: async (input, init) => {
      requests.push({ url: requestUrl(input), init })
      active += 1
      maximumActive = Math.max(maximumActive, active)
      await new Promise((resolve) => setTimeout(resolve, 5))
      active -= 1
      return matrixResponse()
    },
  })

  assert.equal(result.ok, true)
  if (!result.ok) {
    return
  }

  assert.ok(maximumActive > 1)
  assert.deepEqual(
    requests.map(({ url }) => [
      url.searchParams.get("start"),
      url.searchParams.get("end"),
      url.searchParams.get("step"),
    ]),
    [
      ["100", "160", "60"],
      ["100", "160", "60"],
      ["100", "160", "60"],
      ["100", "160", "60"],
    ]
  )
  assert.ok(requests.every(({ url }) => url.pathname === "/api/v1/query_range"))
  assert.ok(requests.every(({ init }) => init?.cache === "no-store"))
  assert.ok(requests.every(({ init }) => init?.signal instanceof AbortSignal))

  assert.deepEqual(result.data, {
    version: 1,
    start: 100,
    end: 160,
    step_seconds: 60,
    series: [
      {
        group: "group-a",
        lane: "lane-a",
        series: "series-a",
        status: "ok",
        points: [
          [100, 1],
          [130, 2],
        ],
      },
      {
        group: "group-a",
        lane: "lane-b",
        series: "series-b",
        status: "ok",
        points: [
          [100, 1],
          [130, 2],
        ],
      },
      {
        group: "group-a",
        lane: "lane-b",
        series: "series-c",
        status: "ok",
        points: [
          [100, 1],
          [130, 2],
        ],
      },
      {
        group: "group-b",
        lane: "lane-c",
        series: "series-d",
        status: "ok",
        points: [
          [100, 1],
          [130, 2],
        ],
      },
    ],
  })
})

test("contains partial upstream failures without exposing upstream details", async () => {
  const result = await getMetricsSnapshot(queryParams(), config, baseUrl, {
    fetcher: async (input) => {
      const query = requestUrl(input).searchParams.get("query")
      if (query === "expr-a") {
        return new Response("upstream URL and query should stay private", {
          status: 502,
        })
      }
      if (query === "expr-b") {
        throw new Error("private upstream failure details")
      }
      if (query === "expr-c") {
        return new Response("not json", {
          headers: { "content-type": "application/json" },
        })
      }
      return matrixResponse()
    },
  })

  assert.equal(result.ok, true)
  if (!result.ok) {
    return
  }

  assert.deepEqual(
    result.data.series.map(({ status }) => status),
    ["error", "error", "error", "ok"]
  )
  assert.deepEqual(result.data.series[0], {
    group: "group-a",
    lane: "lane-a",
    series: "series-a",
    status: "error",
  })
  assert.deepEqual(result.data.series[3], {
    group: "group-b",
    lane: "lane-c",
    series: "series-d",
    status: "ok",
    points: [
      [100, 1],
      [130, 2],
    ],
  })

  const serialized = JSON.stringify(result)
  assert.equal(serialized.includes("private"), false)
  assert.equal(serialized.includes("upstream"), false)
  assert.equal(serialized.includes("expr-a"), false)
})

test("returns a successful envelope when every upstream series fails", async () => {
  const result = await getMetricsSnapshot(queryParams(), config, baseUrl, {
    fetcher: async () => {
      throw new Error("private total failure")
    },
  })

  assert.equal(result.ok, true)
  if (!result.ok) {
    return
  }

  assert.equal(result.data.series.length, 4)
  assert.ok(result.data.series.every(({ status }) => status === "error"))
  assert.ok(
    result.data.series.every((series) => Object.keys(series).length === 4)
  )
})

test("cancels upstream queries when the snapshot request is aborted", async () => {
  const controller = new AbortController()
  const signals: AbortSignal[] = []
  const resultPromise = getMetricsSnapshot(queryParams(), config, baseUrl, {
    signal: controller.signal,
    fetcher: async (_input, init) => {
      const signal = init?.signal
      assert.ok(signal)
      signals.push(signal)

      return new Promise<never>((_resolve, reject) => {
        signal.addEventListener("abort", () => reject(signal.reason), {
          once: true,
        })
      })
    },
  })

  controller.abort()
  const result = await resultPromise

  assert.equal(signals.length, 4)
  assert.ok(signals.every((signal) => signal.aborted))
  assert.equal(result.ok, true)
  if (result.ok) {
    assert.ok(result.data.series.every(({ status }) => status === "error"))
  }
})

test("rejects malformed and oversized snapshot requests before querying", async () => {
  let fetchCount = 0
  const fetcher = async () => {
    fetchCount += 1
    return matrixResponse()
  }

  const invalidStep = await getMetricsSnapshot(
    new URLSearchParams({ start: "100", end: "160", step: "15" }),
    config,
    baseUrl,
    { fetcher }
  )
  assert.deepEqual(invalidStep, {
    ok: false,
    status: 400,
    message: "Invalid metric step.",
  })

  const oversized = await getMetricsSnapshot(
    new URLSearchParams({ start: "0", end: "40960", step: "10" }),
    config,
    baseUrl,
    { fetcher }
  )
  assert.deepEqual(oversized, {
    ok: false,
    status: 400,
    message: "Invalid metric range.",
  })

  const unknownParameter = await getMetricsSnapshot(
    new URLSearchParams({ start: "100", end: "160", step: "30", query: "x" }),
    config,
    baseUrl,
    { fetcher }
  )
  assert.deepEqual(unknownParameter, {
    ok: false,
    status: 400,
    message: "Invalid metric request.",
  })
  assert.equal(fetchCount, 0)
})
