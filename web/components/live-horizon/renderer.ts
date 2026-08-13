import type { TimeSeriesPoint } from "./model"

export type HorizonMode = "offset" | "mirror"

export type HorizonFrame = {
  points: readonly TimeSeriesPoint[]
  start: number
  end: number
  stepSeconds: number
  width: number
  height: number
  pixelRatio: number
  bands: number
  extent: readonly [minimum: number, maximum: number]
  positiveColors: readonly string[]
  negativeColors: readonly string[]
  mode: HorizonMode
  overlapSteps: number
}

export function incrementalRedraw(
  previousEnd: number | null,
  end: number,
  stepSeconds: number,
  width: number,
  overlapSteps: number
) {
  if (previousEnd === null) {
    return null
  }

  const shiftColumns = (end - previousEnd) / stepSeconds
  if (
    !Number.isInteger(shiftColumns) ||
    shiftColumns < 0 ||
    shiftColumns >= width
  ) {
    return null
  }

  return {
    shiftColumns,
    redrawFrom: Math.max(0, width - Math.max(overlapSteps, shiftColumns)),
  }
}

function bandColor(colors: readonly string[], index: number) {
  return colors[Math.min(index, colors.length - 1)]
}

function physicalEdge(value: number, pixelRatio: number) {
  return Math.round(value * pixelRatio)
}

export class HorizonRenderer {
  private readonly canvas: HTMLCanvasElement
  private readonly buffer: HTMLCanvasElement
  private previousEnd: number | null = null
  private previousSignature = ""

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas
    this.buffer = document.createElement("canvas")
  }

  reset() {
    this.previousEnd = null
    this.previousSignature = ""
  }

  render(frame: HorizonFrame) {
    const width = Math.max(1, Math.floor(frame.width))
    const height = Math.max(1, Math.floor(frame.height))
    const pixelRatio = Math.max(1, frame.pixelRatio)
    const physicalWidth = physicalEdge(width, pixelRatio)
    const physicalHeight = physicalEdge(height, pixelRatio)
    const signature = [
      width,
      height,
      pixelRatio,
      frame.bands,
      frame.extent.join(","),
      frame.positiveColors.join(","),
      frame.negativeColors.join(","),
      frame.mode,
      frame.stepSeconds,
    ].join("|")

    const resized =
      this.canvas.width !== physicalWidth ||
      this.canvas.height !== physicalHeight

    if (resized) {
      this.canvas.width = physicalWidth
      this.canvas.height = physicalHeight
      this.buffer.width = physicalWidth
      this.buffer.height = physicalHeight
    }

    const incremental =
      !resized && signature === this.previousSignature
        ? incrementalRedraw(
            this.previousEnd,
            frame.end,
            frame.stepSeconds,
            width,
            frame.overlapSteps
          )
        : null
    const context = this.canvas.getContext("2d")

    if (!context) {
      return
    }

    let redrawFrom = 0
    if (incremental) {
      const { shiftColumns } = incremental
      if (shiftColumns > 0) {
        this.shiftLeft(shiftColumns, pixelRatio)
      }
      redrawFrom = incremental.redrawFrom
    }

    const redrawLeft = physicalEdge(redrawFrom, pixelRatio)
    context.clearRect(redrawLeft, 0, physicalWidth - redrawLeft, physicalHeight)

    const values = new Map(frame.points)
    for (let x = redrawFrom; x < width; x += 1) {
      const timestamp = frame.start + x * frame.stepSeconds
      const value = values.get(timestamp)

      if (value !== undefined) {
        this.drawColumn(context, x, value, frame)
      }
    }

    this.previousEnd = frame.end
    this.previousSignature = signature
  }

  private shiftLeft(columns: number, pixelRatio: number) {
    const shift = physicalEdge(columns, pixelRatio)
    const remaining = this.canvas.width - shift
    const bufferContext = this.buffer.getContext("2d")
    const context = this.canvas.getContext("2d")

    if (!bufferContext || !context || remaining <= 0) {
      return
    }

    bufferContext.clearRect(0, 0, this.buffer.width, this.buffer.height)
    bufferContext.drawImage(
      this.canvas,
      shift,
      0,
      remaining,
      this.canvas.height,
      0,
      0,
      remaining,
      this.canvas.height
    )
    context.clearRect(0, 0, this.canvas.width, this.canvas.height)
    context.drawImage(this.buffer, 0, 0)
  }

  private drawColumn(
    context: CanvasRenderingContext2D,
    x: number,
    value: number,
    frame: HorizonFrame
  ) {
    const [minimum, maximum] = frame.extent
    const pixelRatio = Math.max(1, frame.pixelRatio)
    const left = physicalEdge(x, pixelRatio)
    const right = physicalEdge(x + 1, pixelRatio)
    const width = Math.max(1, right - left)
    const height = physicalEdge(frame.height, pixelRatio)

    if (value > 0 && maximum > 0) {
      const bandSize = maximum / frame.bands
      for (let band = 0; band < frame.bands; band += 1) {
        const amount = Math.min(
          1,
          Math.max(0, (value - band * bandSize) / bandSize)
        )
        if (amount === 0) {
          break
        }

        const fillHeight = Math.max(1, Math.round(amount * height))
        context.fillStyle = bandColor(frame.positiveColors, band)
        context.fillRect(left, height - fillHeight, width, fillHeight)
      }
    }

    if (value < 0 && minimum < 0) {
      const bandSize = Math.abs(minimum) / frame.bands
      for (let band = 0; band < frame.bands; band += 1) {
        const amount = Math.min(
          1,
          Math.max(0, (-value - band * bandSize) / bandSize)
        )
        if (amount === 0) {
          break
        }

        const fillHeight = Math.max(1, Math.round(amount * height))
        context.fillStyle = bandColor(frame.negativeColors, band)
        context.fillRect(
          left,
          frame.mode === "mirror" ? height - fillHeight : 0,
          width,
          fillHeight
        )
      }
    }
  }
}
