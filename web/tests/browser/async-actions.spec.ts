import { expect, test } from "@playwright/test"
import type { Page, Route } from "@playwright/test"

async function editDevice(page: Page) {
  await page.goto("/networks/devices")
  await page
    .getByRole("button", { name: "Edit Example laptop", exact: true })
    .click()
  return page.getByRole("dialog", { name: "Edit device" })
}
async function hold(page: Page, path: string) {
  let resolve!: (route: Route) => void
  const requested = new Promise<Route>((r) => (resolve = r))
  let release!: () => void
  const released = new Promise<void>((r) => (release = r))
  await page.route(path, async (route) => {
    resolve(route)
    await released
    await route
      .fulfill({ json: { password: "generated-forest-river-moon" } })
      .catch(() => {})
  })
  return { requested, release }
}

test("Generate leaves the complete dialog usable; typing wins over a late response", async ({
  page,
}) => {
  const dialog = await editDevice(page)
  const pending = await hold(page, "**/wifi-password/generate")
  await dialog
    .getByRole("button", { name: "Generate password", exact: true })
    .click()
  await pending.requested
  await expect(
    dialog.getByRole("button", { name: "Save changes", exact: true })
  ).toBeEnabled()
  await expect(
    dialog.getByRole("button", { name: "Cancel", exact: true })
  ).toBeEnabled()
  await expect(
    dialog.getByRole("button", { name: "Close", exact: true })
  ).toBeEnabled()
  await dialog
    .getByLabel("Wi-Fi password", { exact: true })
    .fill("manually-entered-password")
  pending.release()
  await expect(
    dialog.getByLabel("Wi-Fi password", { exact: true })
  ).toHaveValue("manually-entered-password")
})

test("Save sends visible values while generation is pending", async ({
  page,
}) => {
  const dialog = await editDevice(page)
  const pending = await hold(page, "**/wifi-password/generate")
  await dialog
    .getByRole("button", { name: "Generate password", exact: true })
    .click()
  await pending.requested
  let payload: Record<string, unknown> = {}
  await page.route("**/bff/v1/networks/devices/*", async (route) => {
    payload = route.request().postDataJSON()
    await route.fulfill({ status: 200, json: {} })
  })
  await dialog
    .getByRole("button", { name: "Save changes", exact: true })
    .click()
  await expect(dialog).not.toBeVisible()
  pending.release()
  expect(payload).not.toHaveProperty("wifi_password")
})

test("Cancel and reopening discard pending generation", async ({ page }) => {
  let dialog = await editDevice(page)
  const pending = await hold(page, "**/wifi-password/generate")
  await dialog
    .getByRole("button", { name: "Generate password", exact: true })
    .click()
  await pending.requested
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click()
  await expect(dialog).not.toBeVisible()
  await page
    .getByRole("button", { name: "Edit Example laptop", exact: true })
    .click()
  dialog = page.getByRole("dialog", { name: "Edit device" })
  pending.release()
  await expect(
    dialog.getByLabel("Wi-Fi password", { exact: true })
  ).toHaveValue("")
})

test("one session request does not disable another session", async ({
  page,
}) => {
  await page.goto("/profile/security")
  const pending = await hold(page, "**/sessions/session-0/revoke")
  await page
    .getByRole("button", { name: "Sign out First client", exact: true })
    .click()
  await pending.requested
  await expect(
    page.getByRole("button", { name: "Sign out Second client", exact: true })
  ).toBeEnabled()
  pending.release()
})

for (const entry of [
  {
    url: "/networks/devices",
    name: "Example laptop",
    menu: "Delete",
    confirm: "Delete device",
    path: "**/bff/v1/networks/devices/*",
  },
  {
    url: "/networks/vlans",
    name: "Private",
    menu: "Delete",
    confirm: "Delete VLAN",
    path: "**/bff/v1/networks/vlans/*",
  },
  {
    url: "/gizmos",
    name: "Example laptop",
    menu: "Remove",
    confirm: "Remove Gizmo",
    path: "**/bff/v1/gizmos/*",
  },
]) {
  test(`${entry.url} deletion errors remain inline`, async ({ page }) => {
    await page.goto(entry.url)
    await page
      .getByRole("button", {
        name: "More actions for " + entry.name,
        exact: true,
      })
      .click()
    await page.getByRole("menuitem", { name: entry.menu, exact: true }).click()
    await page.route(entry.path, (route) =>
      route.fulfill({ status: 503, json: { message: "Try again later." } })
    )
    const dialog = page.getByRole("alertdialog")
    await dialog
      .getByRole("button", { name: entry.confirm, exact: true })
      .click()
    await expect(dialog).toBeVisible()
    await expect(dialog.getByRole("alert")).toContainText("Try again later.")
    await expect(
      dialog.getByRole("button", { name: "Cancel", exact: true })
    ).toBeEnabled()
  })
}

test("audit sorting remains usable during navigation", async ({
  page,
  isMobile,
}) => {
  await page.goto("/admin/audit-logs")
  if (isMobile) {
    await page.getByRole("button", { name: /Sort by/ }).click()
    await page.getByRole("button", { name: "Actor", exact: true }).click()
    await page.getByRole("button", { name: "Apply", exact: true }).click()
    await expect(
      page.getByRole("button", { name: /Sort by Actor/ })
    ).toBeEnabled()
  } else {
    await page.getByRole("button", { name: /Actor/ }).click()
    await expect(page.getByRole("button", { name: /Timestamp/ })).toBeEnabled()
  }
  await expect(page).toHaveURL(/sort_by=actor_username/)
  await expect(
    page.getByRole("button", { name: "Go to next page" })
  ).toBeEnabled()
})

for (const action of ["show", "copy"] as const) {
  test(`${action} keeps the footer usable during a delayed reveal`, async ({
    page,
  }) => {
    await page.addInitScript(() => {
      Object.defineProperty(navigator, "clipboard", {
        value: {
          write: async (items: ClipboardItem[]) => {
            await items[0].getType("text/plain")
          },
        },
      })
    })
    const dialog = await editDevice(page)
    const pending = await hold(page, "**/wifi-password/reveal")
    await dialog
      .getByRole("button", {
        name: action === "show" ? "Show password" : "Copy password",
        exact: true,
      })
      .click()
    await pending.requested
    await expect(
      dialog.getByRole("button", { name: "Save changes" })
    ).toBeEnabled()
    await expect(
      dialog.getByRole("button", { name: "Cancel", exact: true })
    ).toBeEnabled()
    pending.release()
    if (action === "copy")
      await expect(dialog.getByRole("status")).toHaveText("Password copied")
    else
      await expect(
        dialog.getByLabel("Wi-Fi password", { exact: true })
      ).toHaveValue("generated-forest-river-moon")
  })
}

test("copy denial is shown inline", async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      value: {
        write: async () => {
          throw new Error("Clipboard denied")
        },
      },
    })
  })
  const dialog = await editDevice(page)
  await dialog
    .getByLabel("Wi-Fi password", { exact: true })
    .fill("visible-password")
  await dialog
    .getByRole("button", { name: "Copy password", exact: true })
    .click()
  await expect(dialog).toContainText("Clipboard denied")
})

test("actual saving guards remain and the close button stays visible", async ({
  page,
}) => {
  const dialog = await editDevice(page)
  const pending = await hold(page, "**/bff/v1/networks/devices/*")
  await dialog
    .getByRole("button", { name: "Save changes", exact: true })
    .click()
  await pending.requested
  await expect(
    dialog.getByRole("button", { name: "Cancel", exact: true })
  ).toBeDisabled()
  await expect(
    dialog.getByRole("button", { name: "Close", exact: true })
  ).toBeVisible()
  await expect(
    dialog.getByRole("button", { name: "Close", exact: true })
  ).toBeDisabled()
  pending.release()
})

test("bulk sign-out failure stays in its confirmation", async ({ page }) => {
  await page.goto("/profile/security")
  await page
    .getByRole("button", { name: "Sign out all client sessions", exact: true })
    .click()
  await page.route("**/sessions/revoke_others", (route) =>
    route.fulfill({ status: 503, json: { message: "Failed" } })
  )
  const dialog = page.getByRole("alertdialog")
  await dialog
    .getByRole("button", { name: "Sign out all", exact: true })
    .click()
  await expect(dialog.getByRole("alert")).toContainText("Session update failed")
  await expect(
    dialog.getByRole("button", { name: "Cancel", exact: true })
  ).toBeEnabled()
})

for (const entry of [
  { url: "/networks/vlans", name: "Private" },
  { url: "/gizmos", name: "Example laptop" },
]) {
  test(`${entry.url} keeps close visible during saving`, async ({ page }) => {
    await page.goto(entry.url)
    await page
      .getByRole("button", { name: "Edit " + entry.name, exact: true })
      .click()
    const dialog = page.getByRole("dialog")
    const pending = await hold(page, "**/bff/v1/**")
    await dialog
      .getByRole("button", { name: "Save changes", exact: true })
      .click()
    await pending.requested
    await expect(
      dialog.getByRole("button", { name: "Close", exact: true })
    ).toBeVisible()
    await expect(
      dialog.getByRole("button", { name: "Close", exact: true })
    ).toBeDisabled()
    pending.release()
  })
}

test("section save consistently guards the visible close button", async ({
  page,
}) => {
  await page.goto("/")
  await page.getByRole("button", { name: "Sections", exact: true }).click()
  const dialog = page.getByRole("dialog", { name: "Sections" })
  const pending = await hold(page, "**/bff/v1/preferences")
  await dialog.getByRole("button", { name: "Save", exact: true }).click()
  await pending.requested
  await expect(
    dialog.getByRole("button", { name: "Close", exact: true })
  ).toBeVisible()
  await expect(
    dialog.getByRole("button", { name: "Close", exact: true })
  ).toBeDisabled()
  pending.release()
})

test("audit metadata copy uses inline feedback", async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: async () => {} },
    })
  })
  await page.goto("/admin/audit-logs")
  await page
    .getByRole("button", {
      name: "View details for update by tester",
      exact: true,
    })
    .click()
  const dialog = page.getByRole("dialog")
  await dialog.getByRole("button", { name: "Copy", exact: true }).click()
  await expect(dialog.getByRole("status")).toHaveText("Metadata copied")
  await expect(page.locator('[data-slot="toast"]')).toHaveCount(0)
})

test("repeated Generate starts only one request and turning private access off wins", async ({
  page,
}) => {
  const dialog = await editDevice(page)
  let count = 0
  const pending = await hold(page, "**/wifi-password/generate")
  page.on("request", (request) => {
    if (request.url().endsWith("/wifi-password/generate")) count++
  })
  await dialog
    .getByRole("button", { name: "Generate password", exact: true })
    .evaluate((button) => {
      ;(button as HTMLButtonElement).click()
      ;(button as HTMLButtonElement).click()
    })
  await pending.requested
  expect(count).toBe(1)
  await dialog
    .getByRole("checkbox", { name: "Give this device its own Wi-Fi password" })
    .uncheck()
  pending.release()
  await expect(
    dialog.getByLabel("Wi-Fi password", { exact: true })
  ).not.toBeVisible()
})

test("audit page size and sorting compose while a response is pending", async ({
  page,
  isMobile,
}) => {
  if (isMobile) await page.setViewportSize({ width: 1280, height: 900 })
  await page.goto("/admin/audit-logs")
  await page.getByRole("combobox", { name: "Rows per page" }).click()
  await page.getByRole("option", { name: "50", exact: true }).click()
  await page.getByRole("button", { name: /Actor/ }).click()
  await expect(page).toHaveURL(/limit=50/)
  await expect(page).toHaveURL(/sort_by=actor_username/)
})

test("a new device still requires a password before saving", async ({
  page,
}) => {
  await page.goto("/networks/devices")
  await page.getByRole("button", { name: "Add Device", exact: true }).click()
  const dialog = page.getByRole("dialog", { name: "Create device" })
  await dialog.getByLabel("Display name", { exact: true }).fill("New device")
  await dialog
    .getByLabel("MAC address", { exact: true })
    .fill("02:00:00:00:00:02")
  await dialog
    .getByRole("checkbox", { name: "Give this device its own Wi-Fi password" })
    .check()
  const pending = await hold(page, "**/wifi-password/generate")
  await dialog
    .getByRole("button", { name: "Generate password", exact: true })
    .click()
  await pending.requested
  await dialog
    .getByRole("button", { name: "Create device", exact: true })
    .click()
  expect(
    await dialog
      .getByLabel("Wi-Fi password", { exact: true })
      .evaluate((input) => (input as HTMLInputElement).validity.valueMissing)
  ).toBe(true)
  await expect(dialog).toBeVisible()
  pending.release()
})
