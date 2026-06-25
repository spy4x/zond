/**
 * Zond configuration loader
 *
 * Config sources (highest priority first):
 *   1. ZOND_TARGETS env var — comma-separated name=url,name=url
 *   2. ZOND_CONFIG_PATH env var — path to YAML config file
 *   3. ./zond.yaml in working directory
 */

import { parse as parseYaml } from "@std/yaml"

export interface Target {
  name: string
  url: string
  /** Timeout in ms (default 5000) */
  timeout?: number
}

export interface Config {
  port: number
  targets: Target[]
}

const DEFAULT_PORT = 8080
const DEFAULT_TIMEOUT = 5000

export async function loadConfig(): Promise<Config> {
  const portEnv = Deno.env.get("ZOND_PORT")
  const port = portEnv ? parseInt(portEnv, 10) : DEFAULT_PORT

  // 1. Try ZOND_TARGETS env var
  const targetsEnv = Deno.env.get("ZOND_TARGETS")
  if (targetsEnv) {
    const targets = targetsEnv.split(",").map((pair) => {
      const eq = pair.indexOf("=")
      if (eq === -1) {
        throw new Error(
          `Invalid ZOND_TARGETS entry: "${pair}". Expected name=url format.`,
        )
      }
      return {
        name: pair.slice(0, eq).trim(),
        url: pair.slice(eq + 1).trim(),
        timeout: DEFAULT_TIMEOUT,
      }
    })
    return { port, targets }
  }

  // 2. Try config file
  // Default: ./zond.yml. Fallback: ./zond.yaml.
  let configPath = Deno.env.get("ZOND_CONFIG_PATH")
  if (!configPath) {
    try {
      await Deno.stat("./zond.yml")
      configPath = "./zond.yml"
    } catch {
      configPath = "./zond.yaml"
    }
  }

  try {
    const yaml = await Deno.readTextFile(configPath)
    const data = parseYaml(yaml) as {
      port?: number
      targets?: Array<{ name: string; url: string; timeout?: number }>
    }

    if (!data?.targets || !Array.isArray(data.targets)) {
      throw new Error(
        `No targets found in ${configPath}. Add a "targets" array.`,
      )
    }

    return {
      port: data.port || port,
      targets: data.targets.map((t) => ({
        name: t.name,
        url: t.url,
        timeout: t.timeout || DEFAULT_TIMEOUT,
      })),
    }
  } catch (err) {
    if (err instanceof Deno.errors.NotFound) {
      throw new Error(
        `No config found. Set ZOND_TARGETS env var or create a zond.yaml file.`,
      )
    }
    throw err
  }
}
