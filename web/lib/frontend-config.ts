import "server-only"

function readConfigValue(value: string | undefined) {
  const trimmed = value?.trim()
  return trimmed && trimmed.length > 0 ? trimmed : null
}

export function getFrontendSsoConfig() {
  return {
    logoutUrl: readConfigValue(process.env.INTERN_SSO_LOGOUT_URL),
    settingsUrl: readConfigValue(process.env.INTERN_SSO_SETTINGS_URL),
  }
}
