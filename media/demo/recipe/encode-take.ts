// Encodes the last raw take into media/demo/: the title and end cards, captions.vtt burned in,
// music.opus under it. Runs alone after a caption or music change, without recording again:
//
//   deno run -A media/demo/recipe/encode-take.ts [take dir, default media/demo/.take]
import { encodeDemo } from "./encode.ts"

const demo = new URL(`..`, import.meta.url).pathname.replace(/\/$/, ``)
const takeDir = Deno.args[0] ?? `${demo}/.take`

await encodeDemo({
  take: `${takeDir}/frames.txt`,
  out: demo,
  title: `zond`,
  // The README's first line, cut to fit one line of the title card.
  claim: `Health checks behind an SSO proxy, without a monitoring login`,
  end: `github.com/spy4x/zond`,
  poster: `${takeDir}/poster.png`,
})
