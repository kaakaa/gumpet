# gumpet

A desktop pet that shows you your messages.

`gum` is `ugm` rearranged — **u**'ve **g**ot **m**essage, after *You've Got Mail*.
Send it some text over HTTP and a gopher pops up a speech balloon with it.

gumpet is a cross-platform take on [mattn/gopher](https://github.com/mattn/gopher),
which does the same thing on Windows only.

## Install

Download the archive for your platform from the [releases page][releases] and
put `gumpet` and `gumpetctl` somewhere on your `PATH`. The macOS build is a
universal binary and runs on both Apple Silicon and Intel.

Nothing is code-signed, so macOS quarantines the binaries on first run. Clear
that with:

```
xattr -d com.apple.quarantine gumpet gumpetctl
```

[releases]: https://github.com/kaakaa/gumpet/releases

Or install from source:

```
go install github.com/kaakaa/gumpet/cmd/gumpet@latest
go install github.com/kaakaa/gumpet/cmd/gumpetctl@latest
```

While the repository is private, `go install` needs
`GOPRIVATE=github.com/kaakaa/*` and a git credential that can reach it.

Or build from a checkout:

```
make build            # bin/gumpet and bin/gumpetctl
make windows          # cross-compile to bin/windows-amd64/, from any host
```

Ebitengine needs no cgo on Windows, so `GOOS=windows go build` works straight
from macOS or Linux. Go 1.25 or later is required.

## Use

Start the pet:

```
gumpet
```

A gopher appears in the bottom-right corner of your screen and walks around.
Click it for a menu — settings, a test message, and the toggles you reach for
most. Send it something:

```
gumpetctl おなかすいた
gumpetctl -d 30 "the deploy is done"
go test ./... 2>&1 | tail -1 | gumpetctl -
```

Change how it looks and behaves, from the pet's menu or straight from a shell:

```
gumpetctl -settings
```

Or skip the client and talk to the endpoint yourself:

```
curl -X POST http://127.0.0.1:8787/api/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{"text": "hello gopher", "duration_sec": 10}'
```

A plain-text body works too, for when JSON is more ceremony than you want:

```
curl -X POST http://127.0.0.1:8787/api/v1/messages --data-binary 'hello gopher'
```

## HTTP API

The server listens on `127.0.0.1:8787` by default.

| Method | Path                 | Body                                     |
| ------ | -------------------- | ---------------------------------------- |
| `POST` | `/api/v1/messages`   | `{"text": "…", "duration_sec": 10}`, or plain text |
| `GET`  | `/api/v1/messages`   | — (what has been received, newest first) |
| `GET`  | `/api/v1/config`     | —                                        |
| `PUT`  | `/api/v1/config`     | A whole or partial config, as JSON       |
| `GET`  | `/api/v1/healthz`    | —                                        |
| `GET`  | `/`                  | the settings page                        |
| `GET`  | `/messages`          | the messages page                        |

`duration_sec` is optional and overrides `message.duration_sec` for that one
message. Set `server.token` in the config to require a token, sent as
`Authorization: Bearer <token>` or `X-Gumpet-Token: <token>`.

## Configuration

Click the pet and pick **Settings…**, or run `gumpetctl -settings`, to open a
settings page in your browser. Saving from it applies straight away — no
restart, except for the listen address and the Windows taskbar setting, which
are fixed once the process starts. It also has a box for firing a test message
at the pet, which is the quickest way to get the size and position right.

Everything the page does is equally available by editing the config file, which
is written on first run with every setting commented:

- macOS: `~/Library/Application Support/gumpet/config.yaml`
- Windows: `%AppData%\gumpet\config.yaml`
- Linux: `~/.config/gumpet/config.yaml`

`$GUMPET_CONFIG` or `-config` points at a different file, which is how you run
two pets side by side. `gumpet -print-config` prints the defaults.

Restart gumpet after editing it by hand. Saving from the settings page rewrites
the file, comments and all. The settings worth knowing about:

| Setting | Default | What it does |
| ------- | ------- | ------------ |
| `stage.display` | 1 | Which monitor to put the pet on, counting from 1 |
| `stage.fullscreen` | `false` | Let the pet roam that monitor |
| `stage.width` / `stage.height` | 520 × 360 | Otherwise, how big a patch of screen it keeps to |
| `stage.anchor` | `bottom-right` | Which corner that patch sits in — or `custom` with `stage.x` / `stage.y` |
| `behavior.mode` | `always` | `faded` dims the pet when idle, `on-message` hides it until something arrives |
| `behavior.idle_opacity` | 0.35 | How solid a `faded` pet is when it has nothing to say |
| `behavior.roam` | `horizontal` | `none`, `horizontal`, `perimeter` or `wander` — see below |
| `behavior.speed` | 45 | Walking speed, pixels per second |
| `pet.source` | `gopher` | A bundled pet by name, or your own artwork — see below |
| `pet.scale` | 1.0 | Relative to how the artwork is meant to be drawn |
| `pet.smooth` | `auto` | `auto`, `on` or `off` — how the artwork is enlarged |
| `message.max_visible` | 3 | How many balloons may be on screen at once |
| `message.max_width` | 640 | How wide the balloon may grow before the text wraps |
| `message.text_scale` | 1.5 | Message text size, relative to a 12px base |
| `font.path` | *(a system font)* | A .ttf, .otf or .ttc to draw text with |
| `font.system` | `true` | Whether to look for a font on this machine at all |
| `window.click_through` | `false` | Let clicks pass through — at the cost of the pet's menu |
| `history.max` | 200 | How many messages the messages page remembers |
| `history.hours` | 24 | How long it keeps them; 0 for no time limit |

Run with `GUMPET_DEBUG=1` to outline the window, which shows exactly how much of
the screen the pet is covering.

## How the pet gets around

`behavior.roam` picks one of four:

| | |
| ------------ | --- |
| `none` | Stands where it is. |
| `horizontal` | Walks back and forth along the bottom of the stage. |
| `perimeter` | Walks round and round the edge of the stage. |
| `wander` | Heads off in whatever direction it fancies, turning at the walls. |

The pet starts somewhere random on the stage each time gumpet runs, so a
restart does not always put it back in the same corner.

With more than one monitor, `stage.display` picks which one it lives on,
counting from 1 in the order the system reports them — gumpet lists what it
found in its log at startup. It stays on that one: Ebitengine reports each
monitor's size but not where it sits relative to the others, so there is no
coordinate space in which a pet could walk from one screen to the next.

The **stage** is the part of the monitor the pet is allowed into, not a window.
gumpet's window is only as big as the pet and whatever it is saying, and it
moves around the screen to follow the pet — so `stage.fullscreen: true` lets the
pet cross the whole display without a screen-sized rectangle sitting over your
desktop.

## Between messages

`behavior.mode` decides what the pet does with itself when it has nothing to say:

| | |
| ------------ | --- |
| `always` | Stays on screen as it is. |
| `faded` | Stays on screen but goes faint, at `behavior.idle_opacity`. It comes back to full whenever it speaks or you open its menu, so it is still there to click. |
| `on-message` | Disappears. The window turns click-through while it is gone, so an invisible pet never swallows a click. |

`idle_opacity` runs from just above 0 to 1. Around 0.35 leaves a clear gopher
you can still read straight through; much below 0.2 and it is barely there.

## Messages

Messages go into a queue and the pet works through it oldest first. When
several arrive at once it says several at once: up to `message.max_visible`
balloons pile up above it, each offset to one side, so a burst looks like a
crowd talking rather than a tidy queue. The oldest of them is the one at the
bottom, with the tail.

Clicking a balloon takes it down, so a message that has been read does not have
to be waited out. Whatever is next in the queue moves up into its place. The pet
holds still while the cursor is over a balloon, so it stays where you aimed.

Everything gumpet is sent is listed at
[http://127.0.0.1:8787/messages](http://127.0.0.1:8787/messages), newest first,
with whether the pet has said it yet. That covers messages still waiting their
turn, and ones that arrived while the pet was too busy to take them. The page
has a search box that narrows the list as you type, matches highlighted, and a
time range to go with it — the last 10 minutes through to the last 3 days.

The list is kept in memory, so it starts empty every time gumpet runs.
`history.max` and `history.hours` decide how much of it is kept; whichever
limit bites first wins.

## Claude Code

[contrib/claude-code](contrib/claude-code) has a hook that points
[Claude Code](https://claude.com/claude-code) at your pet: the gopher asks you
Claude's questions and tells you when it has finished working, so you can leave
the terminal and still know when you are needed.

## The menu

Clicking the pet opens a menu with:

- which gumpet this is, and the address it is listening on
- **Say something** — a test message, for checking size and placement
- **Settings…** — opens the settings page in your browser
- **Walk** — cycles through the four roaming styles
- **Roam the whole screen**, **Always on top** and **Hide until a message** — toggles
- **Quit**

Changes made here are written to the config file and show up on the settings
page, and the other way round.

The pet stops walking while the cursor is on it or on one of its balloons, so
neither is a moving target however fast it is going. The menu closes when you pick something
from it, when you click elsewhere on the pet, or a few seconds after the cursor
leaves it.

Clicks reach the pet only because `window.click_through` is off, which is the
default. The trade-off is that the window's rectangle — the pet, plus its
balloon while one is up — swallows clicks that land on it. Turn
`window.click_through` on if you would rather it never did; **the menu then
stops opening**, and `gumpetctl -settings` is the way in. gumpet says so in its
log at startup when that setting is on, because there is otherwise no way to
tell a click-through pet from a broken one.

## Choosing a pet

Five pets are bundled. Pick one from the pet's own menu, from the settings
page, or by name in `pet.source`:

| | |
| --- | --- |
| `gopher` | the original walking gopher, and the default |
| `pixel` | a pixel-art gopher, running |
| `astro` | something small and green in a space helmet |
| `rose` | the same idea, in pink |
| `flier` | a winged thing, flapping |

All of them move. A pet that stands still is a picture stuck to the desktop.

`pet.scale` means the same thing for all of them: 1 is how that artwork is
meant to look, whether it was drawn at twelve pixels or five thousand.

A name wins over a file of the same spelling. Write `./pink` if you mean the
file.

## Using your own pet

`pet.source` also accepts:

- **An animated GIF.** Every frame is used, with the delays stored in the file.
- **A single PNG or JPEG.** A pet that does not animate.
- **A directory of images.** Sorted by file name and played in that order.
  Frames named `talk.*` are played while a message is up, if you want the pet
  to do something different while it talks.

Message text is never drawn onto the artwork itself: gumpet draws its own
speech balloon above the pet, sized to the text, whatever the pet looks like.
`message.max_width` caps how wide it grows before wrapping.

Artwork drawn facing left is mirrored when the pet walks right. Set
`pet.flip_when_facing_right: false` if yours should never be mirrored.

`pet.smooth` decides how the artwork is enlarged. `auto` lets the artwork
decide, which is right for everything bundled. Pixel art wants `off`: smoothing
it only blurs the squares it is drawn from, which is the same reason the
bundled bitmap font is never smoothed either.

## Fonts

Text is drawn with a font from this machine, at whatever size
`message.text_scale` works out to, so it is smooth rather than an enlarged
bitmap. gumpet looks for a face that covers Japanese as well as Latin:
Hiragino Sans on macOS, Yu Gothic or Meiryo on Windows, Noto Sans CJK on Linux.

Point `font.path` at a `.ttf`, `.otf` or `.ttc` to use something else. A
collection's first face is the one used, which is the regular weight in all of
the above. Setting `font.system: false` with no path falls back to the bundled
[bitmap font](https://github.com/hajimehoshi/bitmapfont), which is always
available and needs no file at all, but goes blocky when enlarged.

Nothing is bundled beyond that fallback: a Japanese font is several megabytes,
and every desktop gumpet runs on already has one.

## Notes

- The window is transparent, undecorated, always on top, and hides itself from
  the Windows taskbar. It is only as big as the pet and whatever is above it,
  and moves with the pet rather than the pet moving inside it. In `on-message`
  mode it turns click-through until there is something to show, so an invisible
  pet never swallows a click.
- The settings page is served by gumpet itself and loads nothing from the
  network, so it works offline.

## Issues become pull requests

Opening an issue starts [a workflow](.github/workflows/implement-issue.yml) that
reads it, makes the change, and opens a **draft** pull request against it. The
issue forms ask for the three things that decide an implementation — what it
should do, why, and how you would know it works — and the last of those is what
the change gets built against.

Two separate things have to be set up, and the workflow fails at its last step
until both are. Running `/install-github-app` in Claude Code does both at once.

1. **The [Claude GitHub App](https://github.com/apps/claude), installed on this
   repository.** This is how the run authenticates to GitHub: reading the
   issue, pushing a branch, opening the pull request.
2. **A repository secret**, which is how it authenticates to Anthropic, and
   which decides how the tokens are billed:

   | | |
   | --- | --- |
   | `CLAUDE_CODE_OAUTH_TOKEN` | From `claude setup-token`. Bills to a Claude subscription |
   | `ANTHROPIC_API_KEY` | From the Claude Console. Bills as API usage |

Each run also spends GitHub Actions minutes for the runner it executes on,
which is GitHub's billing rather than Anthropic's.

It only runs for issues opened by someone with write access, since the issue
text drives an agent that can push here. Label an issue `no-auto` to keep it
from running, or run the workflow by hand against an issue number to try again.

Told to build something the issue does not describe well enough, it is meant to
comment saying what it would need rather than guess — a guessed implementation
costs more to review than none.

## Releasing

Pushing a tag builds every platform and publishes the archives:

```
git tag v0.1.0
git push origin v0.1.0
```

The workflow runs `go vet` and the tests first, then builds Windows (amd64 and
arm64) and Linux by cross-compiling from a Linux runner, and macOS on a macOS
runner because Metal needs cgo. It attaches the archives and a `SHA256SUMS` file
to the release.

## License

MIT — see [LICENSE](LICENSE).

### The artwork

Every bundled pet carries a `NOTICE` naming where it came from and under what
terms. Two things are always separate there: the licence on the drawing, and
the licence on the gopher itself, which is Renée French's regardless of who
drew any particular one.

| Pet | Drawing | |
| --- | --- | --- |
| `gopher` | [mattn/gopher](https://github.com/mattn/gopher), MIT | [NOTICE](assets/gopher/NOTICE) |
| `pixel` | [egonelbre/gophers](https://github.com/egonelbre/gophers), CC0 1.0 | [NOTICE](assets/pixel/NOTICE) |
| `astro`, `rose`, `flier` | [Kenney](https://kenney.nl/assets/pixel-platformer), CC0 1.0 | [NOTICE](assets/astro/NOTICE) |

The two gophers are of a character created by Renée French, used under
[CC BY 3.0](https://creativecommons.org/licenses/by/3.0/). CC0 waives rights in
a drawing; it does not waive anything in the character that drawing is of. The
Kenney sprites are nobody's character but their own.

The bundled images have been enlarged by a whole number with nearest-neighbour
sampling, so their pixels stay square, and the multi-tile ones assembled into
animations. They are otherwise unchanged.
