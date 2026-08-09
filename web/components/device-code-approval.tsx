"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"

import { buildBffPath } from "@/lib/bff"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSeparator,
  InputOTPSlot,
} from "@/components/ui/input-otp"

type ApiError = {
  code?: string
  message?: string
}

type ParsedApiError =
  | { kind: "auth"; message: string }
  | { kind: "error"; message: string }

const sessionsPath = "/profile/security?tab=mine"

function normalizeUserCode(value: string) {
  return value.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, 8)
}

function formatUserCode(value: string) {
  const normalized = normalizeUserCode(value)

  if (normalized.length <= 4) {
    return normalized
  }

  return `${normalized.slice(0, 4)}-${normalized.slice(4)}`
}

async function parseApiError(response: Response): Promise<ParsedApiError> {
  let body: ApiError | null = null

  try {
    body = (await response.json()) as ApiError
  } catch {
    body = null
  }

  if (response.status >= 300 && response.status < 400) {
    return {
      kind: "auth",
      message:
        "You need an authenticated browser session to approve or deny a device login request.",
    }
  }

  if (response.status === 404) {
    return {
      kind: "error",
      message:
        "No pending login request was found for that code. Check the code or start the login flow again on the client to get a new one.",
    }
  }

  if (response.status === 409) {
    return {
      kind: "error",
      message:
        "This code is no longer pending. It may already be approved, denied, exchanged, or expired. Start the login flow again on the client for a new code.",
    }
  }

  if (response.status === 401) {
    return {
      kind: "auth",
      message:
        "You need an authenticated browser session to approve or deny a device login request.",
    }
  }

  if (response.status === 400 && body?.message === "user_code must not be empty") {
    return {
      kind: "error",
      message: "Enter the full 8-character device approval code.",
    }
  }

  if (body?.message) {
    return {
      kind: "error",
      message: body.message,
    }
  }

  return {
    kind: "error",
    message: `${response.status} ${response.statusText}`,
  }
}

export function DeviceCodeApproval({
  initialUserCode,
}: {
  initialUserCode: string
}) {
  const router = useRouter()
  const [userCode, setUserCode] = useState(normalizeUserCode(initialUserCode))
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [busyAction, setBusyAction] = useState<"approve" | "deny" | null>(null)
  const formattedUserCode = formatUserCode(userCode)
  const approvalPath =
    formattedUserCode.length > 0
      ? `/auth/device?user_code=${encodeURIComponent(formattedUserCode)}`
      : "/auth/device"

  async function submit(action: "approve" | "deny") {
    const normalizedUserCode = normalizeUserCode(userCode)
    if (normalizedUserCode.length !== 8) {
      setSubmitError("Enter the full 8-character device approval code.")
      return
    }

    const formattedUserCode = formatUserCode(normalizedUserCode)

    setBusyAction(action)
    setSubmitError(null)

    let response: Response

    try {
      response = await fetch(
        buildBffPath(
          `/auth/device_codes/${encodeURIComponent(formattedUserCode)}/${action}`
        ),
        {
          method: "POST",
        }
      )
    } catch {
      window.location.assign(approvalPath)
      return
    }

    if (!response.ok) {
      setBusyAction(null)
      const error = await parseApiError(response)
      if (error.kind === "auth") {
        window.location.assign(approvalPath)
        return
      }

      setSubmitError(error.message)
      return
    }

    setBusyAction(null)
    toast.success(action === "approve" ? "Device login approved." : "Device login denied.")
    router.push(sessionsPath)
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4 py-8 lg:px-6">
      <Card className="w-full max-w-lg border bg-card shadow-xs">
        <CardHeader>
          <CardTitle>Device Approval</CardTitle>
          <CardDescription>
            Approve or deny a pending login request from the displayed user
            code.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="grid gap-6"
            onSubmit={(event) => {
              event.preventDefault()
              void submit("approve")
            }}
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="device-user-code">User code</FieldLabel>
                <InputOTP
                  id="device-user-code"
                  maxLength={8}
                  value={userCode}
                  pasteTransformer={normalizeUserCode}
                  onChange={(value) => {
                    setUserCode(normalizeUserCode(value))
                    if (submitError) {
                      setSubmitError(null)
                    }
                  }}
                  containerClassName="justify-center"
                  autoFocus
                >
                  <InputOTPGroup>
                    <InputOTPSlot index={0} />
                    <InputOTPSlot index={1} />
                    <InputOTPSlot index={2} />
                    <InputOTPSlot index={3} />
                  </InputOTPGroup>
                  <InputOTPSeparator />
                  <InputOTPGroup>
                    <InputOTPSlot index={4} />
                    <InputOTPSlot index={5} />
                    <InputOTPSlot index={6} />
                    <InputOTPSlot index={7} />
                  </InputOTPGroup>
                </InputOTP>
                <FieldDescription>
                  Paste the full code into the first box or type it manually.
                </FieldDescription>
              </Field>
              <FieldError>{submitError}</FieldError>
            </FieldGroup>
            <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
              <Button
                type="button"
                variant="outline"
                disabled={busyAction !== null}
                onClick={() => void submit("deny")}
              >
                {busyAction === "deny" ? "Denying..." : "Deny"}
              </Button>
              <Button type="submit" disabled={busyAction !== null}>
                {busyAction === "approve" ? "Approving..." : "Approve"}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
