#!/usr/bin/env deno run -A
/**
 * Zond — Internal health probe bridge
 *
 * Receives external health check requests and forwards them to
 * internal Docker containers via their Docker DNS names.
 * Returns 200 or 503 — no sensitive data exposed.
 *
 * Endpoints:
 *   GET /              — list all targets
 *   GET /health        — list all targets
 *   GET /health/:name  — check a single target
 */

import { loadConfig, type Config } from "./config.ts"
import { checkTarget } from "./health.ts"

async function main() {
  const config: Config = await loadConfig()

  Deno.serve({ port: config.port }, async (req) => {
    const url = new URL(req.url)
    const path = url.pathname

    const corsHeaders = {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, OPTIONS",
      "Access-Control-Allow-Headers": "*",
    }

    if (req.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: corsHeaders })
    }

    // GET /health/{name} — check a single target
    const healthMatch = path.match(/^\/health\/([a-zA-Z0-9_-]+)$/)
    if (healthMatch) {
      const name = healthMatch[1]
      const target = config.targets.find((t) => t.name === name)
      if (!target) {
        return new Response(`unknown target: ${name}\n`, {
          status: 404,
          headers: { ...corsHeaders, "Content-Type": "text/plain" },
        })
      }

      const ok = await checkTarget(target)

      return new Response(ok ? "ok\n" : "unreachable\n", {
        status: ok ? 200 : 503,
        headers: { ...corsHeaders, "Content-Type": "text/plain" },
      })
    }

    // GET / or /health — list all targets
    if (path === "/" || path === "/health") {
      const results = await Promise.all(
        config.targets.map(async (t) => {
          const ok = await checkTarget(t)
          return { name: t.name, url: t.url, ok }
        }),
      )

      const lines = results.map((r) =>
        `${r.ok ? "OK" : "DOWN"} ${r.name} ${r.url}`
      )
      const allOk = results.every((r) => r.ok)

      return new Response(lines.join("\n") + "\n", {
        status: allOk ? 200 : 503,
        headers: { ...corsHeaders, "Content-Type": "text/plain" },
      })
    }

    return new Response("not found\n", {
      status: 404,
      headers: { ...corsHeaders, "Content-Type": "text/plain" },
    })
  })
}

if (import.meta.main) {
  main()
}
