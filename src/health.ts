/**
 * Zond — target health check logic
 */

import type { Target } from "./config.ts"

/**
 * Check a target's health by making an HTTP GET request.
 * Returns true if the target responds with a 2xx or 3xx status.
 */
export async function checkTarget(target: Target): Promise<boolean> {
  try {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), target.timeout ?? 5000)

    const resp = await fetch(target.url, {
      method: "GET",
      signal: controller.signal,
      headers: {
        "User-Agent": "Zond/1.0",
        "Accept": "*/*",
      },
    })

    clearTimeout(timer)
    return resp.status >= 200 && resp.status < 400
  } catch {
    return false
  }
}
