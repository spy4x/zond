// Records the raw take of zond's demo video: a real terminal (ttyd, started by record.sh) and then a
// small browser page that shows zond's real answers. Run by record.sh, with the staged stack's
// stand-ins up and ttyd serving the demo shell:
//
//   deno run -A media/demo/recipe/record.ts <ttyd url> <take dir>
//
// The take is held screenshots, not a screen recording: after every change (a key, a pointer step,
// an answer) the page is captured and the frame is given a scripted duration. A loaded machine
// makes the recording slower but never changes the video's timing, and the Web Animations on the
// page (the click mark, the key cap) are stepped by the same clock. The one real duration is zond's
// answer time, which the browser part shows as a counter that never runs ahead of the real wait.
//
// It writes <take dir>/frames.txt (an ffmpeg concat list), poster.png and marks.txt: when each
// step starts, in seconds of the finished video (after the 2 s title card), to write captions.vtt.
import { chromium } from "playwright"
import type { Page } from "playwright"

const W = 1280
const H = 800
const FPS = 30
const INK = `#15120f`
const ZOND = `http://localhost:8080`
/** Finished-video seconds before the take starts: the title card. */
const TITLE_SECONDS = 2
const [ttydUrl, takeDir] = Deno.args
if (!ttydUrl || !takeDir) {
  throw new Error(`usage: record.ts <ttyd url> <take dir>`)
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))
const ease = (t: number) => t < 0.5 ? 4 * t ** 3 : 1 - (-2 * t + 2) ** 3 / 2

/** The take: screenshots, each with the time it stays on screen. */
class Take {
  frames: { file: string; ms: number }[] = []
  marks: string[] = []

  /** Milliseconds of take so far. */
  get ms(): number {
    return this.frames.reduce((sum, frame) => sum + frame.ms, 0)
  }

  /** Notes that step `name` starts now, in seconds of the finished video. */
  mark(name: string) {
    this.marks.push(`${((this.ms / 1000) + TITLE_SECONDS).toFixed(2)} ${name}`)
  }

  /**
   * Captures the page as it is and keeps it on screen for `ms`. Every running animation is paused
   * and set to the take's clock first, then moved on by `ms`, so it plays at the video's pace.
   */
  async shot(page: Page, ms: number) {
    await page.evaluate((step) => {
      for (const animation of document.getAnimations()) {
        const a = animation as Animation & { demoT?: number }
        a.pause()
        const t = a.demoT ?? 0
        const end = Number(a.effect?.getComputedTiming().endTime ?? 0)
        if (t >= end) a.finish()
        else a.currentTime = t
        a.demoT = t + step
      }
    }, ms)
    const file = `${takeDir}/${String(this.frames.length).padStart(5, `0`)}.jpg`
    await page.screenshot({ path: file, type: `jpeg`, quality: 92 })
    this.frames.push({ file, ms })
  }

  /** Keeps the last frame on screen `ms` longer. Nothing may be moving. */
  hold(ms: number) {
    const last = this.frames.at(-1)
    if (!last) throw new Error(`hold before the first frame`)
    last.ms += ms
  }

  /**
   * Writes the take as an ffconcat list. Each still is read at 30 fps (an image is otherwise read
   * at 25 fps and every start rounds to 40 ms), and each still starts on the frame nearest its
   * scripted time, so short holds (a key, a pointer step) average out to their scripted pace.
   */
  async write() {
    const lines = [`ffconcat version 1.0`]
    let at = 0
    let frame = 0
    for (const { file, ms } of this.frames) {
      at += ms
      const next = Math.max(frame + 1, Math.round(at * FPS / 1000))
      lines.push(
        `file '${file}'`,
        `option framerate ${FPS}`,
        `duration ${((next - frame) / FPS).toFixed(6)}`,
      )
      frame = next
    }
    // The concat demuxer ignores the last entry's duration, so the last file comes once more.
    lines.push(`file '${this.frames.at(-1)!.file}'`, `option framerate ${FPS}`)
    await Deno.writeTextFile(`${takeDir}/frames.txt`, `${lines.join(`\n`)}\n`)
    await Deno.writeTextFile(
      `${takeDir}/marks.txt`,
      `${this.marks.join(`\n`)}\n`,
    )
    console.log(
      `take: ${this.frames.length} frames, ${(this.ms / 1000).toFixed(2)} s`,
    )
    console.log(this.marks.join(`\n`))
  }
}

const take = new Take()

// --- The terminal part --------------------------------------------------------------------------

/** The terminal's visible text with the cursor position: changes whenever the screen does. */
function screen(page: Page): Promise<string> {
  return page.evaluate(() => {
    // deno-lint-ignore no-explicit-any
    const term = (globalThis as any).term
    const buffer = term.buffer.active
    const rows = [`${buffer.cursorX},${buffer.cursorY}`]
    for (let i = 0; i < term.rows; i++) {
      rows.push(
        buffer.getLine(buffer.viewportY + i)?.translateToString(true) ?? ``,
      )
    }
    return rows.join(`\n`)
  })
}

/**
 * Waits until the terminal differs from `before`, passes `ready` and has stopped changing for
 * `quiet` ms, then until xterm.js has painted it.
 */
async function settle(
  page: Page,
  before: string,
  { ready = () => true, quiet = 80, timeout = 10_000 }: {
    ready?: (text: string) => boolean
    quiet?: number
    timeout?: number
  } = {},
): Promise<string> {
  const deadline = Date.now() + timeout
  let now = await screen(page)
  while (now === before || !ready(now)) {
    if (Date.now() > deadline) {
      throw new Error(`the terminal did not change:\n${now}`)
    }
    await sleep(15)
    now = await screen(page)
  }
  for (let still = await screen(page);; still = await screen(page)) {
    await sleep(quiet)
    const again = await screen(page)
    if (again === still) break
    if (Date.now() > deadline) {
      throw new Error(`the terminal did not settle:\n${again}`)
    }
  }
  await page.evaluate(() =>
    new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)))
  )
  return await screen(page)
}

const PROMPT = /~\/stack \$\s*$/
const promptReady = (text: string) => {
  const lines = text.split(`\n`)
  const [, y] = lines[0].split(`,`).map(Number)
  return PROMPT.test(lines[y + 1] ?? ``)
}

/** Types `text` a key at a time, `ms` a key, each key's echo captured. */
async function typeKeys(page: Page, text: string, ms: number) {
  for (const key of text) {
    const before = await screen(page)
    await page.keyboard.type(key)
    await settle(page, before)
    await take.shot(page, ms)
  }
}

/** Presses `key`, shows `cap` as a key cap when given, and captures the answer. */
async function press(
  page: Page,
  key: string,
  ms: number,
  cap?: string,
  ready?: typeof promptReady,
) {
  const before = await screen(page)
  if (cap) await page.evaluate((label) => globalThis.__demoKey(label), cap)
  await page.keyboard.press(key)
  await settle(page, before, {
    ready,
    quiet: ready ? 400 : 80,
    timeout: 60_000,
  })
  await take.shot(page, ms)
}

/** Hides the key cap, letting its fade play. */
async function hideCap(page: Page) {
  await page.evaluate(() => globalThis.__demoKeyHide())
  for (let i = 0; i < 4; i++) await take.shot(page, 33)
}

/** Runs `command` without recording it, and waits for the prompt. */
async function offCamera(page: Page, command: string) {
  await page.keyboard.type(command)
  await page.keyboard.press(`Enter`)
  await sleep(300)
  // The screen after a `clear` can equal the one before it, so no change is required here.
  await settle(page, `-`, { ready: promptReady, quiet: 300 })
}

async function recordTerminal(page: Page) {
  await page.goto(ttydUrl)
  await page.waitForFunction(() => `term` in globalThis)
  await page.addStyleTag({
    content: `html, body { background: ${INK} !important; margin: 0; overflow: hidden }
      #terminal-container { position: fixed !important; inset: 32px 32px 104px 36px !important;
        width: auto !important; height: auto !important; background: ${INK} !important }
      .xterm-viewport { overflow: hidden !important }`,
  })
  await page.evaluate(() => globalThis.__demoPointerVisible(false))
  // deno-lint-ignore no-explicit-any
  await page.evaluate(() => (globalThis as any).term.fit())
  await settle(page, ``, { ready: promptReady, quiet: 300 })
  await page.locator(`.xterm-helper-textarea`).focus()
  // deno-lint-ignore no-explicit-any
  const cols = await page.evaluate(() => (globalThis as any).term.cols)
  console.log(`terminal: ${cols} columns`)

  // 1. The config watches one service.
  take.mark(`cat`)
  await take.shot(page, 900)
  await typeKeys(page, `cat zond.yml`, 60)
  take.hold(300)
  await press(page, `Enter`, 3000, undefined, promptReady)
  await page.screenshot({ path: `${takeDir}/poster.png` })

  // 2. Add ollama and grafana, as in zond.example.yml.
  take.mark(`nano`)
  await typeKeys(page, `nano zond.yml`, 60)
  take.hold(300)
  await press(page, `Enter`, 1400)
  for (let i = 0; i < 5; i++) await press(page, `ArrowDown`, 160, `↓`)
  await hideCap(page)
  take.hold(300)
  const lines = [
    `  - name: ollama`,
    `    url: http://hl-ollama:11434/api/tags`,
    `    timeout: 10000`,
    `  - name: grafana`,
    `    url: http://hl-grafana:3000/api/health`,
    `    timeout: 3000`,
  ]
  for (const [i, line] of lines.entries()) {
    await typeKeys(page, line, 50)
    if (i < lines.length - 1) await press(page, `Enter`, 200)
  }
  take.hold(900)
  // Save and close.
  take.mark(`save`)
  await press(page, `Control+o`, 1000, `Ctrl O`)
  await press(page, `Enter`, 1000, `Enter`)
  await press(page, `Control+x`, 900, `Ctrl X`, promptReady)
  await hideCap(page)
  take.hold(500)
  await offCamera(page, `clear`)

  // 3. Start zond next to the services.
  take.mark(`up`)
  await take.shot(page, 600)
  await typeKeys(page, `docker compose up -d`, 60)
  take.hold(300)
  await press(page, `Enter`, 2800, undefined, promptReady)

  // 4. Ask it about metube.
  take.mark(`curl`)
  await typeKeys(page, `curl -i localhost:8080/health/metube`, 60)
  take.hold(300)
  await press(page, `Enter`, 3500, undefined, promptReady)
}

// --- The browser part ---------------------------------------------------------------------------

/** Where the pointer is; `moveTo` updates it. */
const pointer = { x: 940, y: 420 }

/** Glides the pointer to `to` along a slight curve, one captured step a frame. */
async function moveTo(page: Page, to: { x: number; y: number }) {
  const from = { ...pointer }
  const dx = to.x - from.x
  const dy = to.y - from.y
  const dist = Math.hypot(dx, dy)
  const steps = Math.round(Math.min(700, Math.max(400, dist * 0.8)) / 33)
  const bow = Math.min(40, dist * 0.08)
  const nx = dist ? -dy / dist : 0
  const ny = dist ? dx / dist : 0
  for (let i = 1; i <= steps; i++) {
    const t = i / steps
    const arc = Math.sin(Math.PI * t) * bow
    await page.mouse.move(
      from.x + dx * ease(t) + nx * arc,
      from.y + dy * ease(t) + ny * arc,
    )
    await take.shot(page, 33)
  }
  Object.assign(pointer, to)
}

/**
 * Clicks the address bar like a person (pause, press, hold, release), right of where the URL ends,
 * so the pointer never covers the text typed next.
 */
async function clickAddress(page: Page) {
  const box = (await page.locator(`#url`).boundingBox())!
  const x = Math.round(box.x + box.width - 60)
  await moveTo(page, { x, y: Math.round(box.y + box.height / 2) })
  take.hold(150)
  await page.mouse.down()
  for (let i = 0; i < 6; i++) await take.shot(page, 33)
  await page.mouse.up()
  for (let i = 0; i < 8; i++) await take.shot(page, 33)
}

/**
 * Loads `url` in the drawn browser and shows zond's real answer: the counter runs at the video's
 * pace but never ahead of the real wait, the status shows the real response's code, and the page
 * below is the real response.
 */
async function load(page: Page, url: string) {
  let answer: { at: number; text: string; ok: boolean } | null = null
  const onResponse = (
    res: { url(): string; status(): number; statusText(): string },
  ) => {
    if (!res.url().startsWith(ZOND)) return
    answer = {
      at: Date.now(),
      text: `${res.status()} ${res.statusText()}`,
      ok: res.status() < 400,
    }
  }
  page.on(`response`, onResponse)
  // The Enter listener must be in place before Enter is pressed.
  await page.evaluate((target) => globalThis.demoLoad(target), `http://${url}`)
  const sent = Date.now()
  await page.keyboard.press(`Enter`)
  let shown = 0
  while (true) {
    const got = answer as { at: number } | null
    const real = (got?.at ?? Date.now()) - sent
    if (got && shown >= real) break
    if (!got && real > 30_000) throw new Error(`zond did not answer ${url} in 30 s`)
    if (shown + 100 > real && !got) {
      await sleep(10)
      continue
    }
    shown += 100
    await page.evaluate(
      (s) => globalThis.demoWaiting(s),
      (shown / 1000).toFixed(1),
    )
    await take.shot(page, 100)
  }
  await page.evaluate(() => globalThis.demoLoaded)
  const { text, ok } = answer!
  await page.evaluate(
    ([t, o, s]) => globalThis.demoAnswer(t, o, s),
    [
      text,
      ok,
      (shown / 1000).toFixed(1),
    ] as const,
  )
  page.off(`response`, onResponse)
}

async function recordBrowser(page: Page, url: string) {
  await page.goto(url)
  await page.evaluate(() => document.fonts.ready)
  await page.mouse.move(pointer.x, pointer.y)

  // 5. One service that is down.
  take.mark(`grafana`)
  await take.shot(page, 800)
  await clickAddress(page)
  await typeAddress(page, `localhost:8080/health/grafana`)
  take.hold(400)
  take.mark(`grafana-wait`)
  await load(page, `localhost:8080/health/grafana`)
  take.mark(`grafana-answer`)
  await take.shot(page, 900)
  await moveTo(page, { x: 1100, y: 560 })
  take.hold(2600)

  // 6-7. Every service at once.
  take.mark(`all`)
  await clickAddress(page)
  for (let i = 0; i < `/grafana`.length; i++) {
    await page.keyboard.press(`Backspace`)
    await take.shot(page, 80)
  }
  take.hold(400)
  take.mark(`all-wait`)
  await load(page, `localhost:8080/health`)
  take.mark(`all-answer`)
  await take.shot(page, 900)
  await moveTo(page, { x: 1100, y: 560 })
  take.hold(3800)
}

/** Types into the address bar, 60 ms a key. */
async function typeAddress(page: Page, text: string) {
  for (const key of text) {
    await page.keyboard.type(key)
    await take.shot(page, 60)
  }
}

declare global {
  var __demoKey: (label: string) => void
  var __demoKeyHide: () => void
  var __demoPointerVisible: (visible: boolean) => void
  var demoLoad: (url: string) => void
  var demoLoaded: Promise<void>
  var demoWaiting: (seconds: string) => void
  var demoAnswer: (text: string, ok: boolean, seconds: string) => void
}

// Headless Chromium has no address bar, so this page draws one and shows zond's answer in an
// iframe scaled to 250%. A new answer loads into a hidden iframe and replaces the shown one when
// the counter reaches it, so the old answer stays on screen while zond thinks.
const BROWSER_PAGE = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="color-scheme" content="dark">
<title>zond</title>
<style>
  :root { color-scheme: dark; }
  html, body { margin: 0; height: 100%; background: ${INK}; color: #efebe2;
    font: 22px/1.4 "Hack", "DejaVu Sans Mono", monospace; }
  .bar { display: flex; gap: 16px; align-items: center; height: 84px;
    padding: 0 28px; background: #1c1916; border-bottom: 1px solid #413c38;
    box-sizing: border-box; }
  .dots { display: flex; gap: 9px; }
  .dots i { width: 14px; height: 14px; border-radius: 50%; background: #413c38; }
  #url { flex: 1; height: 50px; padding: 0 20px; border-radius: 25px;
    border: 1px solid #413c38; background: ${INK}; color: #efebe2; font: inherit;
    outline: none; caret-color: #efebe2; }
  #url:focus { border-color: #857f79; }
  #url::placeholder { color: #857f79; }
  #status { min-width: 330px; text-align: right; color: #bcb7af; font-size: 20px; }
  #status b { font-weight: 600; }
  .ok { color: #83d494; } .ko { color: #f87e77; }
  .view { position: absolute; inset: 84px 0 0 0; }
  iframe { position: absolute; top: 0; left: 0; border: 0; width: 40%; height: 40%;
    transform: scale(2.5); transform-origin: 0 0; background: ${INK}; color-scheme: dark; }
  iframe.next { visibility: hidden; }
</style></head>
<body>
  <div class="bar">
    <div class="dots" aria-hidden="true"><i></i><i></i><i></i></div>
    <input id="url" placeholder="Type a URL" autocomplete="off" spellcheck="false">
    <div id="status"></div>
  </div>
  <div class="view" id="view"></div>
  <script>
    const status = document.getElementById("status")
    const view = document.getElementById("view")
    let next = null
    // Called by the recorder before it presses Enter; demoLoaded resolves once the answer loads.
    globalThis.demoLoad = (url) => { globalThis.demoLoaded = new Promise((resolve) => {
      document.getElementById("url").addEventListener("keydown", (e) => {
        if (e.key !== "Enter") return
        next = document.createElement("iframe")
        next.className = "next"
        next.title = "zond"
        next.onload = () => resolve()
        next.src = url
        view.append(next)
        document.getElementById("url").blur()
      }, { once: true })
    }) }
    globalThis.demoWaiting = (s) => { status.textContent = "Waiting for localhost… " + s + " s" }
    globalThis.demoAnswer = (text, ok, s) => {
      for (const old of view.querySelectorAll("iframe:not(.next)")) old.remove()
      next.className = ""
      const b = document.createElement("b")
      b.className = ok ? "ok" : "ko"
      b.textContent = text
      status.replaceChildren("HTTP ", b, " · " + s + " s")
    }
  </script>
</body></html>`

/** Records both parts and writes the take. */
async function main() {
  await Deno.mkdir(takeDir, { recursive: true })
  const server = Deno.serve(
    { hostname: `127.0.0.1`, port: 0, onListen: () => {} },
    () =>
      new Response(BROWSER_PAGE, {
        headers: { "content-type": `text/html; charset=utf-8` },
      }),
  )
  const browser = await chromium.launch()
  try {
    const context = await browser.newContext({
      viewport: { width: W, height: H },
      deviceScaleFactor: 1,
      colorScheme: `dark`,
      locale: `en-GB`,
      timezoneId: `UTC`,
    })
    try {
      await context.addInitScript(
        `window.__demoPointerOptions = { x: 940, y: 420, iframes: "shield" };` +
          `window.__demoKeysOptions = { manual: true }`,
      )
      await context.addInitScript({
        path: new URL(`./pointer.js`, import.meta.url).pathname,
      })
      await context.addInitScript({
        path: new URL(`./keys.js`, import.meta.url).pathname,
      })
      const terminal = await context.newPage()
      await recordTerminal(terminal)
      const page = await context.newPage()
      await recordBrowser(page, `http://127.0.0.1:${server.addr.port}/`)
      await take.write()
    } finally {
      await context.close()
    }
  } finally {
    await browser.close()
    await server.shutdown()
  }
}

await main()
