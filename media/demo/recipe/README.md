# zond demo video: the recipe

`media/demo/recipe/record.sh` remakes the demo video in `media/demo/` from this checkout, end to
end, in one command. It takes about 5–15 minutes, most of it the recording on a busy machine; the
video's timing does not depend on how busy the machine is.

## What it needs

- Docker with the compose plugin, and port 8080 free (the video shows `localhost:8080`).
- `ttyd` (1.7 or later), `ss`, and the Hack font (`fc-list | grep Hack`).
- Deno 2, and Playwright's Chromium for Playwright 1.63
  (`deno run -A npm:playwright@1.63.0 install chromium`).
- ffmpeg with libvpx-vp9, libx264, libopus, libwebp and drawtext.

## What it does

1. Builds zond's image from this checkout (`zond-demo/zond:local`), so the video shows the code it
   sits next to.
2. Stages a stack in a scratch folder (`compose.yml`, project `zond-demo`): zond, plus two
   `traefik/whoami` stand-ins for metube and ollama. Grafana is left stopped, so zond reports it
   down. The config starts with metube only (`zond.start.yml`).
3. Serves a clean shell with ttyd: no environment of the recorder's, an empty staged `HOME`, and
   `demo.bashrc` as the only startup file. It prints a rule before every prompt, so each command's
   output stands apart.
4. `record.ts` records the raw take into `media/demo/.take/` (gitignored): the terminal (show the
   config, add two services in nano, start zond, ask about metube), then a drawn browser bar over
   zond's real answers for grafana and for every service. The take is held screenshots: after every
   change the page is captured and given a scripted duration, and the page's animations (click
   marks, key caps) are stepped by the same clock. The one real duration is zond's 3 s timeout for
   grafana, shown by a counter that never runs ahead of the real wait. It prints when each step
   starts (also in `.take/marks.txt`), in seconds of the finished video.
5. `encode-take.ts` runs `encode.ts` (the shared demo-video encoder): the title card, the captions
   from `media/demo/captions.vtt`, the end card, and `media/demo/music.opus` mixed to about -28
   LUFS. It writes `demo.webm`, `demo.mp4`, `poster.webp` and `poster-640.webp`.
6. On every exit path it removes the stack, the image tag, ttyd and its scratch folder.

## Changing a caption or the music only

Edit `media/demo/captions.vtt` or replace `music.opus` (and `MUSIC.md`), then re-encode the last
take without recording again:

```bash
cd media/demo/recipe && deno run -A encode-take.ts
```

The encoder refuses captions shorter than 2.5 s, faster than 15 characters a second or longer than
64 characters, and a mix louder than -27 LUFS or above -1 dBTP.

## Changing what the video shows

Edit `record.ts`, run `record.sh`, then move the cues in `captions.vtt` to the step times it prints
and encode again.
