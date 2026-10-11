export async function mutate(path: string, options: RequestInit) {
  let response: Response
  try {
    response = await fetch(path, options)
  } catch {
    throw new Error("Couldn't reach the server. Try again.")
  }
  if (response.ok) return

  let message = `Request failed (${response.status}). Try again.`
  try {
    const body = await response.json()
    if (typeof body.message === "string" && body.message.trim())
      message = body.message
  } catch {
    // Some proxies return an empty or non-JSON error response.
  }
  throw new Error(message)
}
