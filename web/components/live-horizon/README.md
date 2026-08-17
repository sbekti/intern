# Live Horizon

`LiveHorizon` is a small React and Canvas time-series chart. It has no knowledge
of Prometheus, Intern, or a specific deployment. A caller supplies the current
points, visible range, step, and any site-specific labels, colors, and styling.

By default, the color bands use a symmetric extent derived from the visible
points, matching Cubism's behavior. Pass `extentHeadroom` to pad the automatic
domain, or `extent` to use a fixed domain.

Points are `[unixSeconds, value]` tuples on the configured step. The component
does not fetch data or own polling state. The renderer copies the existing
canvas to the left and redraws the exposed columns instead of repainting the
full history when the controlled range advances.

The canvas stretches the controlled range's samples to the available width
while retaining incremental updates.

Pointer movement controls the ruler. Use the controlled `rulerTimestamp` and
`onRulerTimestampChange` props to synchronize rulers across multiple charts.
Set `interactive` to `false` when a parent surface owns pointer tracking.

To extract it, copy this directory into a React library and export `index.ts`.
The public dependency is React; the renderer uses standard browser Canvas APIs.
