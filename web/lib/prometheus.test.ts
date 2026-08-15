import assert from "node:assert/strict"
import test from "node:test"

import { parsePrometheusMatrix, parsePrometheusStep } from "./prometheus.ts"

test("accepts only allowlisted Prometheus query steps", () => {
  assert.equal(parsePrometheusStep("10", [10, 30, 60, 300]), 10)
  assert.equal(parsePrometheusStep("15", [10, 30, 60, 300]), null)
  assert.equal(parsePrometheusStep("30.5", [10, 30, 60, 300]), null)
  assert.equal(parsePrometheusStep(null, [10, 30, 60, 300]), null)
})

test("parses one ordered Prometheus matrix series", () => {
  const response = {
    status: "success",
    data: {
      resultType: "matrix",
      result: [
        {
          metric: {},
          values: [
            [100, "12.5"],
            [130, "18"],
          ],
        },
      ],
    },
  }

  assert.deepEqual(parsePrometheusMatrix(response), [
    [100, 12.5],
    [130, 18],
  ])
})

test("rejects empty, multiple, or malformed Prometheus series", () => {
  assert.equal(
    parsePrometheusMatrix({
      status: "success",
      data: { resultType: "matrix", result: [] },
    }),
    null
  )
  assert.equal(
    parsePrometheusMatrix({
      status: "success",
      data: {
        resultType: "matrix",
        result: [{ values: [[100, "1"]] }, { values: [[100, "2"]] }],
      },
    }),
    null
  )
  assert.equal(
    parsePrometheusMatrix({
      status: "success",
      data: { resultType: "matrix", result: [{ values: [[100, "NaN"]] }] },
    }),
    null
  )
})
