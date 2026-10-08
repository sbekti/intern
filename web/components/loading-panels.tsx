import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"

export function ProfileLoadingPanel() {
  return (
    <div className="px-4 lg:px-6" aria-label="Loading profile" aria-busy="true">
      <div className="mx-auto grid w-full max-w-3xl gap-4">
        <Card>
          <CardHeader className="flex flex-row items-start gap-4">
            <Skeleton className="size-10 shrink-0 rounded-full" />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <div className="grid gap-1">
                <Skeleton className="h-6 w-36 max-w-full" />
                <Skeleton className="h-5 w-48 max-w-full" />
              </div>
              <Skeleton className="h-5 w-28 rounded-full" />
            </div>
          </CardHeader>
          <CardContent className="@container grid gap-4">
            <div className="grid gap-2 @sm:grid-cols-[8rem_minmax(0,1fr)] @sm:gap-4">
              <Skeleton className="h-5 w-20" />
              <Skeleton className="h-5 w-28" />
            </div>
            <div className="grid gap-2 @sm:grid-cols-[8rem_minmax(0,1fr)] @sm:gap-4">
              <Skeleton className="h-5 w-14" />
              <div className="flex flex-wrap gap-2">
                <Skeleton className="h-5.5 w-28 rounded-full" />
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-20" />
            <Skeleton className="h-5 w-48 max-w-full" />
          </CardHeader>
          <CardContent className="@container grid gap-4">
            {[0, 1].map((row) => (
              <div key={row} className="grid gap-4">
                {row > 0 ? <Separator /> : null}
                <div className="flex flex-col gap-3 @lg:flex-row @lg:items-center @lg:justify-between @lg:gap-6">
                  <div className="grid min-w-0 gap-1">
                    <Skeleton className="h-5 w-24" />
                    <Skeleton className="h-5 w-64 max-w-full" />
                  </div>
                  <Skeleton className="h-9 w-full rounded-md @lg:w-40" />
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
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
  return <TableLoadingPanel kind="audit" />
}

const loadingTables = {
  vlans: {
    title: "VLANs",
    description: "Create, update, and remove VLAN definitions for the network.",
    columns: ["w-16", "w-24", "w-64", "w-20"],
  },
  devices: {
    title: "Devices",
    description:
      "Register devices and assign each MAC address to the correct VLAN.",
    columns: ["w-32", "w-36", "w-20", "w-16", "w-20"],
  },
  gizmos: {
    title: "Gizmos",
    description: "Assign kiosk destinations to registered network devices.",
    columns: ["w-32", "w-36", "w-64", "w-20"],
  },
  audit: {
    title: "Audit Stream",
    description: "Loading entries…",
    columns: ["w-36", "w-20", "w-28", "w-40", "w-56"],
  },
} as const

export function TableLoadingPanel({
  kind,
}: {
  kind: keyof typeof loadingTables
}) {
  const { title, description, columns } = loadingTables[kind]
  return (
    <div className="px-4 lg:px-6">
      <Card
        className="border-border/70 shadow-xs"
        aria-label="Loading records"
        aria-busy="true"
      >
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
          {kind !== "audit" ? (
            <CardAction className="ml-3 sm:ml-0">
              <Skeleton className="size-8 sm:w-28" />
            </CardAction>
          ) : null}
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <Skeleton
            className="h-9 w-full rounded-md"
            data-slot="filter-bar-skeleton"
          />
          <Table>
            <TableHeader>
              <TableRow>
                {columns.map((width, index) => (
                  <TableHead key={index}>
                    <Skeleton
                      className={cn(
                        "h-4",
                        width,
                        kind !== "audit" &&
                          index === columns.length - 1 &&
                          "ml-auto"
                      )}
                    />
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {[0, 1, 2].map((row) => (
                <TableRow key={row}>
                  {columns.map((width, index) => (
                    <TableCell key={index}>
                      <Skeleton
                        className={cn(
                          kind !== "audit" && index === columns.length - 1
                            ? "ml-auto h-8"
                            : "h-4",
                          width
                        )}
                      />
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
        {kind === "audit" ? (
          <CardFooter>
            <div className="flex w-full flex-wrap items-center gap-4">
              <div className="hidden items-center gap-3 lg:flex">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-8 w-20" />
              </div>
              <Skeleton className="h-4 w-24 lg:ml-auto" />
              <div className="ml-auto flex items-center gap-2 lg:ml-0">
                {[0, 1, 2, 3].map((item) => (
                  <Skeleton
                    key={item}
                    className={cn(
                      "size-8",
                      (item === 0 || item === 3) && "hidden lg:block"
                    )}
                  />
                ))}
              </div>
            </div>
          </CardFooter>
        ) : null}
      </Card>
    </div>
  )
}
