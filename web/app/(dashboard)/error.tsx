"use client"

export default function DashboardError({
  error,
}: Readonly<{
  error: Error
}>) {
  return (
    <div className="px-4 lg:px-6">
      <div className="rounded-xl border border-destructive/30 bg-card px-6 py-5 shadow-xs">
        <p className="text-sm font-medium text-destructive">
          Backend required
        </p>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          This frontend requires a reachable `intern-api` backend. The request
          failed instead of falling back to mock content.
        </p>
        <pre className="mt-4 overflow-x-auto rounded-lg bg-muted px-4 py-3 text-xs text-muted-foreground">
          {error.message}
        </pre>
      </div>
    </div>
  )
}
