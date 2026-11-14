"use client"

import { useEffect, useRef, useState } from "react"
import {
  CheckIcon,
  CopyIcon,
  EyeIcon,
  EyeOffIcon,
  RefreshCwIcon,
} from "lucide-react"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldSet,
} from "@/components/ui/field"
import { buildBffPath } from "@/lib/bff"
import { mutate } from "@/lib/mutations"
import { cn } from "@/lib/utils"

export type WiFiPasswordForm = {
  private_wifi_password_enabled: boolean
  wifi_password: string
}

export function DeviceWiFiPassword({
  value,
  onChange,
  deviceId,
  passwordSet,
  disabled,
  signal,
}: {
  value: WiFiPasswordForm
  onChange: (patch: Partial<WiFiPasswordForm>) => void
  deviceId?: string
  passwordSet: boolean
  disabled: boolean
  signal: AbortSignal
}) {
  const [copied, setCopied] = useState(false)
  const [visible, setVisible] = useState(false)
  const [savedPassword, setSavedPassword] = useState<string | null>(null)
  type Action = "generate" | "show" | "copy"
  const requests = useRef(new Map<Action, AbortController>())
  const [pending, setPending] = useState<Set<Action>>(new Set())

  function cancelRequests() {
    for (const controller of requests.current.values()) controller.abort()
    requests.current.clear()
    setPending(new Set())
  }

  useEffect(() => {
    const controllers = requests.current
    return () => {
      for (const controller of controllers.values()) controller.abort()
    }
  }, [signal])
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!copied) return
    const timeout = window.setTimeout(() => setCopied(false), 2000)
    return () => window.clearTimeout(timeout)
  }, [copied])

  async function requestPassword(path: string, requestSignal: AbortSignal) {
    const response = await mutate(buildBffPath(path), {
      method: "POST",
      cache: "no-store",
      signal: AbortSignal.any([
        requestSignal,
        signal,
        AbortSignal.timeout(15_000),
      ]),
    })
    const data: { password: string } = await response.json()
    return data.password
  }

  async function readPassword(requestSignal: AbortSignal) {
    if (value.wifi_password) return value.wifi_password
    if (savedPassword !== null) return savedPassword
    if (!deviceId || !passwordSet) throw new Error("No password saved.")
    const password = await requestPassword(
      `/networks/devices/${deviceId}/wifi-password/reveal`,
      requestSignal
    )
    requestSignal.throwIfAborted()
    signal.throwIfAborted()
    setSavedPassword(password)
    return password
  }

  async function handleAction(action: Action) {
    if (requests.current.has(action) || disabled || signal.aborted) return
    const controller = new AbortController()
    requests.current.set(action, controller)
    setPending((previous) => new Set(previous).add(action))
    setCopied(false)
    setError(null)
    const current = () => !controller.signal.aborted && !signal.aborted
    try {
      if (action === "generate") {
        const password = await requestPassword(
          "/networks/devices/wifi-password/generate",
          controller.signal
        )
        if (!current()) return
        setSavedPassword(null)
        onChange({ wifi_password: password })
      } else if (action === "show") {
        if (!visible) await readPassword(controller.signal)
        if (current()) setVisible(!visible)
      } else {
        await navigator.clipboard.write([
          new ClipboardItem({
            "text/plain": readPassword(controller.signal).then(
              (password) => new Blob([password], { type: "text/plain" })
            ),
          }),
        ])
        if (current()) setCopied(true)
      }
    } catch (error) {
      if (current())
        setError(
          error instanceof Error
            ? error.message
            : "Couldn't access the password."
        )
    } finally {
      if (requests.current.get(action) === controller) {
        requests.current.delete(action)
        setPending((previous) => {
          const next = new Set(previous)
          next.delete(action)
          return next
        })
      }
    }
  }

  const hasPassword = Boolean(value.wifi_password || passwordSet)

  return (
    <FieldSet>
      <Field orientation="horizontal" data-disabled={disabled}>
        <Checkbox
          id="device-private-wifi"
          disabled={disabled}
          checked={value.private_wifi_password_enabled}
          onCheckedChange={(checked) => {
            cancelRequests()
            onChange({ private_wifi_password_enabled: checked })
          }}
        />
        <FieldContent>
          <FieldLabel htmlFor="device-private-wifi">
            Give this device its own Wi-Fi password
          </FieldLabel>
        </FieldContent>
      </Field>
      {value.private_wifi_password_enabled && (
        <Field data-invalid={Boolean(error)}>
          <FieldLabel htmlFor="device-wifi-passphrase">
            Wi-Fi password
          </FieldLabel>
          <InputGroup>
            <InputGroupInput
              id="device-wifi-passphrase"
              name="wifi-passphrase"
              type={visible ? "text" : "password"}
              autoComplete="off"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              pattern="[ -~]{8,63}"
              title="Use 8 to 63 printable ASCII characters."
              required={!passwordSet}
              disabled={disabled}
              aria-invalid={Boolean(error)}
              placeholder={passwordSet ? "Saved password" : "Enter a password"}
              value={value.wifi_password || savedPassword || ""}
              onChange={(event) => {
                cancelRequests()
                setSavedPassword(null)
                onChange({ wifi_password: event.target.value })
              }}
            />
            <InputGroupAddon align="inline-end">
              <Tooltip>
                <TooltipTrigger
                  render={
                    <InputGroupButton
                      size="icon-sm"
                      aria-label={visible ? "Hide password" : "Show password"}
                      disabled={disabled || !hasPassword}
                      aria-disabled={pending.has("show") || undefined}
                      aria-busy={pending.has("show") || undefined}
                      onClick={() => handleAction("show")}
                    />
                  }
                >
                  {visible ? <EyeOffIcon /> : <EyeIcon />}
                </TooltipTrigger>
                <TooltipContent>
                  {visible ? "Hide password" : "Show password"}
                </TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <InputGroupButton
                      size="icon-sm"
                      aria-label="Copy password"
                      disabled={disabled || !hasPassword}
                      aria-disabled={pending.has("copy") || undefined}
                      aria-busy={pending.has("copy") || undefined}
                      onClick={() => handleAction("copy")}
                    />
                  }
                >
                  {copied ? <CheckIcon /> : <CopyIcon />}
                </TooltipTrigger>
                <TooltipContent>Copy password</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <InputGroupButton
                      size="icon-sm"
                      aria-label="Generate password"
                      disabled={disabled}
                      aria-disabled={pending.has("generate") || undefined}
                      aria-busy={pending.has("generate") || undefined}
                      onClick={() => handleAction("generate")}
                    />
                  }
                >
                  <RefreshCwIcon
                    className={cn(
                      pending.has("generate") && "motion-safe:animate-spin"
                    )}
                  />
                </TooltipTrigger>
                <TooltipContent>Generate password</TooltipContent>
              </Tooltip>
            </InputGroupAddon>
          </InputGroup>
          {passwordSet && (
            <FieldDescription>
              Leave blank to keep the saved password.
            </FieldDescription>
          )}
          <span role="status" className="sr-only">
            {copied ? "Password copied" : ""}
          </span>
          <FieldError>{error}</FieldError>
        </Field>
      )}
    </FieldSet>
  )
}
