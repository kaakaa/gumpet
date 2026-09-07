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
| `stage.fullscreen` | `false` | Let the pet roam the whole monitor |
| `stage.width` / `stage.height` | 520 × 360 | Otherwise, how big a patch of screen it keeps to |
| `stage.anchor` | `bottom-right` | Which corner that patch sits in — or `custom` with `stage.x` / `stage.y` |
| `behavior.mode` | `always` | `on-message` hides the pet until something arrives |
| `behavior.roam` | `horizontal` | `none`, `horizontal`, `perimeter` or `wander` — see below |
| `behavior.speed` | 45 | Walking speed, pixels per second |
| `pet.source` | *(built-in gopher)* | Your own artwork — see below |
| `pet.scale` | 1.0 | The built-in gopher is 200 × 200 |
| `message.max_visible` | 3 | How many balloons may be on screen at once |
| `message.max_width` | 520 | How wide the balloon may grow before the text wraps |
| `message.text_scale` | 2.0 | Message text size, relative to the font's own 12px |
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

The **stage** is the part of the monitor the pet is allowed into, not a window.
gumpet's window is only as big as the pet and whatever it is saying, and it
moves around the screen to follow the pet — so `stage.fullscreen: true` lets the
pet cross the whole display without a screen-sized rectangle sitting over your
desktop.

## Messages

Messages go into a queue and the pet works through it oldest first. When
several arrive at once it says several at once: up to `message.max_visible`
balloons pile up above it, each offset to one side, so a burst looks like a
crowd talking rather than a tidy queue. The oldest of them is the one at the
bottom, with the tail.

Everything gumpet is sent is listed at
[http://127.0.0.1:8787/messages](http://127.0.0.1:8787/messages), newest first,
with whether the pet has said it yet. That covers messages still waiting their
turn, and ones that arrived while the pet was too busy to take them.

The list is kept in memory, so it starts empty every time gumpet runs.
`history.max` and `history.hours` decide how much of it is kept; whichever
limit bites first wins.

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

The pet stops walking while the cursor is on it, so one crossing the screen at
speed is still something you can click. The menu closes when you pick something
from it, when you click elsewhere on the pet, or a few seconds after the cursor
leaves it.

Clicks reach the pet only because `window.click_through` is off, which is the
default. The trade-off is that the window's rectangle — the pet, plus its
balloon while one is up — swallows clicks that land on it. Turn
`window.click_through` on if you would rather it never did; **the menu then
stops opening**, and `gumpetctl -settings` is the way in. gumpet says so in its
log at startup when that setting is on, because there is otherwise no way to
tell a click-through pet from a broken one.

## Using your own pet

`pet.source` accepts:

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

## Notes

- Text is rendered with [bitmapfont](https://github.com/hajimehoshi/bitmapfont),
  which covers Japanese, so no font has to be installed or bundled.
- The window is transparent, undecorated, always on top, and hides itself from
  the Windows taskbar. It is only as big as the pet and whatever is above it,
  and moves with the pet rather than the pet moving inside it. In `on-message`
  mode it turns click-through until there is something to show, so an invisible
  pet never swallows a click.
- The settings page is served by gumpet itself and loads nothing from the
  network, so it works offline.

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

The bundled gopher walk cycle comes from [mattn/gopher](https://github.com/mattn/gopher)
(MIT). The gopher character was created by Renée French and the images are used
under [CC BY 3.0](https://creativecommons.org/licenses/by/3.0/); see
[assets/gopher/NOTICE](assets/gopher/NOTICE).
