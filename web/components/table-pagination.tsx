"use client"

import { useRouter } from "next/navigation"
import Link from "next/link"
import { startTransition } from "react"
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  ChevronsLeftIcon,
  ChevronsRightIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Field, FieldLabel } from "@/components/ui/field"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

type TablePaginationProps = {
  pageSizeId: string
  limit: number
  offset: number
  total: number
  pageSizes: readonly number[]
  buildHref: (params: { limit: number; offset: number }) => string
}

export function TablePagination({
  pageSizeId,
  limit,
  offset,
  total,
  pageSizes,
  buildHref,
}: TablePaginationProps) {
  const router = useRouter()

  const totalPages = Math.max(1, Math.ceil(total / limit))
  const currentPage =
    total === 0 ? 1 : Math.min(totalPages, Math.floor(offset / limit) + 1)
  const previousOffset = Math.max(0, offset - limit)
  const nextOffset = offset + limit
  const lastOffset = total === 0 ? 0 : Math.max(0, (totalPages - 1) * limit)
  const hasPrevious = offset > 0
  const hasNext = nextOffset < total

  function goTo(nextLimit: number, nextOffset: number) {
    startTransition(() => {
      router.push(buildHref({ limit: nextLimit, offset: nextOffset }))
    })
  }

  return (
    <div className="flex w-full flex-wrap items-center gap-4">
      <Field
        orientation="horizontal"
        className="hidden w-fit items-center gap-3 lg:flex"
      >
        <FieldLabel htmlFor={pageSizeId}>Rows per page</FieldLabel>
        <Select
          value={String(limit)}
          onValueChange={(value) => {
            if (!value) {
              return
            }
            goTo(Number(value), 0)
          }}
        >
          <SelectTrigger id={pageSizeId} size="sm" className="w-20">
            <SelectValue />
          </SelectTrigger>
          <SelectContent align="start">
            <SelectGroup>
              {pageSizes.map((size) => (
                <SelectItem key={size} value={String(size)}>
                  {size}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>

      <div className="flex items-center justify-center text-sm font-medium lg:ml-auto">
        Page {currentPage} of {totalPages}
      </div>

      <div className="ml-auto flex items-center gap-2 lg:ml-0">
        <Button
          variant="outline"
          className="hidden size-8 lg:flex"
          size="icon"
          disabled={!hasPrevious}
          nativeButton={hasPrevious ? false : true}
          render={
            hasPrevious ? <Link href={buildHref({ limit, offset: 0 })} /> : undefined
          }
          aria-label="Go to first page"
        >
          <ChevronsLeftIcon />
        </Button>
        <Button
          variant="outline"
          className="size-8"
          size="icon"
          disabled={!hasPrevious}
          nativeButton={hasPrevious ? false : true}
          render={
            hasPrevious ? (
              <Link href={buildHref({ limit, offset: previousOffset })} />
            ) : undefined
          }
          aria-label="Go to previous page"
        >
          <ChevronLeftIcon />
        </Button>
        <Button
          variant="outline"
          className="size-8"
          size="icon"
          disabled={!hasNext}
          nativeButton={hasNext ? false : true}
          render={
            hasNext ? <Link href={buildHref({ limit, offset: nextOffset })} /> : undefined
          }
          aria-label="Go to next page"
        >
          <ChevronRightIcon />
        </Button>
        <Button
          variant="outline"
          className="hidden size-8 lg:flex"
          size="icon"
          disabled={!hasNext}
          nativeButton={hasNext ? false : true}
          render={
            hasNext ? <Link href={buildHref({ limit, offset: lastOffset })} /> : undefined
          }
          aria-label="Go to last page"
        >
          <ChevronsRightIcon />
        </Button>
      </div>
    </div>
  )
}
