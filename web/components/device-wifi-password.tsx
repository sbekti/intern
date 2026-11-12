"use client"

import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldSet,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { toast } from "@/components/ui/toast"
import { buildBffPath } from "@/lib/bff"
import { mutate } from "@/lib/mutations"

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
  onBusyChange,
}: {
  value: WiFiPasswordForm
  onChange: (patch: Partial<WiFiPasswordForm>) => void
  deviceId?: string
  passwordSet: boolean
  disabled: boolean
  onBusyChange: (busy: boolean) => void
}) {
  const [visible, setVisible] = useState(false)
  const [savedPassword, setSavedPassword] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function requestPassword(path: string) {
    const response = await mutate(buildBffPath(path), {
      method: "POST",
      cache: "no-store",
      signal: AbortSignal.timeout(15_000),
    })
    const data: { password: string } = await response.json()
    return data.password
  }

  async function readPassword() {
    if (value.wifi_password) return value.wifi_password
    if (savedPassword !== null) return savedPassword
    if (!deviceId || !passwordSet) throw new Error("No password saved.")
    const password = await requestPassword(
      `/networks/devices/${deviceId}/wifi-password/reveal`
    )
    setSavedPassword(password)
    return password
  }

  async function handleAction(action: "generate" | "show" | "copy") {
    setBusy(true)
    onBusyChange(true)
    setError(null)
    try {
      if (action === "generate") {
        const password = await requestPassword(
          "/networks/devices/wifi-password/generate"
        )
        setSavedPassword(null)
        onChange({ wifi_password: password })
      } else if (action === "show") {
        if (!visible) await readPassword()
        setVisible(!visible)
      } else {
        // Start the clipboard write in the click gesture, including on Safari.
        await navigator.clipboard.write([
          new ClipboardItem({
            "text/plain": readPassword().then(
              (password) => new Blob([password], { type: "text/plain" })
            ),
          }),
        ])
        toast.add({ type: "success", title: "Password copied" })
      }
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Couldn't access the password."
      )
    } finally {
      setBusy(false)
      onBusyChange(false)
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
          onCheckedChange={(checked) =>
            onChange({ private_wifi_password_enabled: checked })
          }
        />
        <FieldContent>
          <FieldLabel htmlFor="device-private-wifi">
            Use private Wi-Fi password
          </FieldLabel>
          <FieldDescription>
            Applies on networks configured for private passwords.
          </FieldDescription>
        </FieldContent>
      </Field>
      {value.private_wifi_password_enabled && (
        <Field data-invalid={Boolean(error)}>
          <FieldLabel htmlFor="device-wifi-passphrase">
            Wi-Fi password
          </FieldLabel>
          <Input
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
            disabled={disabled || busy}
            aria-invalid={Boolean(error)}
            placeholder={passwordSet ? "Saved password" : "Enter a password"}
            value={value.wifi_password || savedPassword || ""}
            onChange={(event) => {
              setSavedPassword(null)
              onChange({ wifi_password: event.target.value })
            }}
          />
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled || busy}
              onClick={() => handleAction("generate")}
            >
              Generate
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled || busy || !hasPassword}
              onClick={() => handleAction("show")}
            >
              {visible ? "Hide" : "Show"}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={disabled || busy || !hasPassword}
              onClick={() => handleAction("copy")}
            >
              Copy
            </Button>
          </div>
          <FieldDescription>
            {passwordSet ? "Leave blank to keep the saved password. " : ""}
            Turning private access off keeps the saved password.
          </FieldDescription>
          <FieldError>{error}</FieldError>
        </Field>
      )}
    </FieldSet>
  )
}
