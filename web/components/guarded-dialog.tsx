"use client"

import { useState, type ReactNode } from "react"
import { Dialog } from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"

export function GuardedDialog({
  open,
  onOpenChange,
  dirty,
  busy,
  children,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  dirty: boolean
  busy: boolean
  children: ReactNode
}) {
  const [confirmDiscard, setConfirmDiscard] = useState(false)
  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!next && busy) return
          if (!next && dirty) setConfirmDiscard(true)
          else onOpenChange(next)
        }}
      >
        {children}
      </Dialog>
      <AlertDialog open={confirmDiscard} onOpenChange={setConfirmDiscard}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard changes?</AlertDialogTitle>
            <AlertDialogDescription>
              Your unsaved changes will be lost.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep editing</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                setConfirmDiscard(false)
                onOpenChange(false)
              }}
            >
              Discard changes
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
