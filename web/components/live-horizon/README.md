# Live Horizon

`LiveHorizon` is a small React and Canvas time-series chart. It has no knowledge
of Prometheus, Intern, or IAD2. A caller supplies an asynchronous loader and any
site-specific labels, colors, and styling.

By default, the color bands use a symmetric extent derived from the visible
points, matching Cubism's behavior. Pass `extentHeadroom` to pad the automatic
domain, or `extent` to use a fixed domain.

The loader returns `[unixSeconds, value]` points on the configured step. After
the initial visible range, the component requests only the newest overlapping
slice. The renderer copies the existing canvas to the left and redraws the
exposed columns instead of repainting the full history.

By default, one sample maps to one CSS pixel, as in Cubism. Set `windowSeconds`
to use an explicit time window; the canvas stretches its configured samples to
the available width while retaining incremental updates.

Pointer movement controls the ruler. Use the controlled `rulerTimestamp` and
`onRulerTimestampChange` props to synchronize rulers across multiple charts.

To extract it, copy this directory into a React library and export `index.ts`.
The public dependency is React; the renderer uses standard browser Canvas APIs.
