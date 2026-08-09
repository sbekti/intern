import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

export function HomeLoadingPanel() {
  return (
    <div className="grid gap-4 px-4 lg:px-6">
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-40" />
          <Skeleton className="h-4 w-full max-w-xs" />
        </CardHeader>
      </Card>
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-28" />
          <Skeleton className="h-4 w-full max-w-xs" />
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-3">
            <Skeleton className="h-28 w-full rounded-xl" />
            <Skeleton className="h-28 w-full rounded-xl" />
            <Skeleton className="h-28 w-full rounded-xl" />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export function ProfileLoadingPanel() {
  return (
    <div className="grid gap-4 px-4 lg:px-6">
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-36" />
          <Skeleton className="h-4 w-48" />
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2">
            <Skeleton className="h-24 w-full rounded-xl" />
            <Skeleton className="h-24 w-full rounded-xl" />
          </div>
        </CardContent>
      </Card>
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-24" />
          <Skeleton className="h-4 w-72" />
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Skeleton className="h-5 w-24 rounded-full" />
          <Skeleton className="h-5 w-32 rounded-full" />
          <Skeleton className="h-5 w-28 rounded-full" />
        </CardContent>
      </Card>
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-24" />
          <Skeleton className="h-4 w-full max-w-sm" />
        </CardHeader>
        <CardContent>
          <Skeleton className="h-9 w-52 rounded-md" />
        </CardContent>
      </Card>
    </div>
  )
}

export function SecurityLoadingPanel({
  isAdmin = false,
}: {
  isAdmin?: boolean
}) {
  return (
    <div className="grid gap-4 px-4 lg:px-6">
      {isAdmin ? (
        <div className="flex items-center gap-3 px-1">
          <Skeleton className="h-9 w-28 rounded-md" />
          <Skeleton className="h-9 w-28 rounded-md" />
        </div>
      ) : null}

      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-32" />
          <Skeleton className="h-4 w-full max-w-lg" />
        </CardHeader>
        <CardContent className="grid gap-3">
          <Skeleton className="h-18 w-full rounded-xl" />
          <Skeleton className="h-18 w-full rounded-xl" />
          <Skeleton className="h-18 w-full rounded-xl" />
        </CardContent>
        {isAdmin ? (
          <CardFooter className="flex w-full flex-col gap-4 md:flex-row md:items-center">
            <div className="flex items-center gap-3">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-9 w-20 rounded-md" />
            </div>
            <Skeleton className="h-4 w-24 md:ml-auto" />
            <div className="ml-auto flex items-center gap-2 lg:ml-0">
              <Skeleton className="hidden size-8 rounded-md lg:block" />
              <Skeleton className="size-8 rounded-md" />
              <Skeleton className="size-8 rounded-md" />
              <Skeleton className="hidden size-8 rounded-md lg:block" />
            </div>
          </CardFooter>
        ) : null}
      </Card>
    </div>
  )
}

export function AuditLogsLoadingPanel() {
  return (
    <div className="grid gap-4 px-4 lg:px-6">
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-16" />
          <Skeleton className="h-4 w-full max-w-md" />
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Skeleton className="h-9 w-full rounded-md" />
            <Skeleton className="h-9 w-full rounded-md" />
            <Skeleton className="h-9 w-full rounded-md" />
            <Skeleton className="h-9 w-full rounded-md" />
          </div>
        </CardContent>
      </Card>

      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className="h-6 w-28" />
          <Skeleton className="h-4 w-full max-w-sm" />
        </CardHeader>
        <CardContent>
          <div className="grid gap-0">
            <div className="grid grid-cols-5 gap-4 px-6 py-3">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-20" />
            </div>
            <div className="grid grid-cols-5 gap-4 border-b px-6 py-4">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-4 w-full" />
            </div>
            <div className="grid grid-cols-5 gap-4 border-b px-6 py-4">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-36" />
              <Skeleton className="h-4 w-full" />
            </div>
            <div className="grid grid-cols-5 gap-4 px-6 py-4">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-4 w-18" />
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-4 w-full" />
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export function TableLoadingPanel({
  titleWidth = "w-24",
}: {
  titleWidth?: string
}) {
  return (
    <div className="px-4 lg:px-6">
      <Card className="border-border/70 shadow-xs">
        <CardHeader className="gap-2">
          <Skeleton className={`h-6 ${titleWidth}`} />
          <Skeleton className="h-4 w-64" />
        </CardHeader>
        <CardContent>
          <div className="grid gap-0">
            <div className="grid grid-cols-4 gap-4 px-6 py-3">
              <Skeleton className="h-4 w-20" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-4 w-16" />
            </div>
            <div className="grid gap-0">
              <div className="grid grid-cols-4 gap-4 border-b px-6 py-4">
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-4 w-14" />
              </div>
              <div className="grid grid-cols-4 gap-4 border-b px-6 py-4">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-36" />
                <Skeleton className="h-4 w-18" />
              </div>
              <div className="grid grid-cols-4 gap-4 px-6 py-4">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-4 w-18" />
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-24" />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
