# gumpet user guide

Everything the [README](../README.md) leaves out.

- [Sending messages](#sending-messages)
- [HTTP API](#http-api)
- [Configuration](#configuration)
- [How the pet gets around](#how-the-pet-gets-around)
- [Between messages](#between-messages)
- [Messages](#messages)
- [Talking to itself](#talking-to-itself)
- [Agents and questions](#agents-and-questions)
- [The menu](#the-menu)
- [Choosing a pet](#choosing-a-pet)
- [Using your own pet](#using-your-own-pet)
- [Fonts](#fonts)
- [Updating](#updating)
- [Notes](#notes)

## Sending messages

`gumpetctl` sends its arguments, or standard input when the argument is `-`:

```
gumpetctl おなかすいた
gumpetctl -d 30 "the deploy is done"
gumpetctl -title CI -level success "All checks have passed"
go test ./... 2>&1 | tail -1 | gumpetctl -
```

| Flag | What it does |
| ---- | ------------ |
| `-d` | Seconds to keep the message up, instead of `message.duration_sec` |
| `-title` | A heading drawn above the message |
| `-level` | `info`, `success`, `warn` or `error`. Colours the balloon, and decides how the pet reacts |
| `-settings` | Opens the settings page instead of sending anything |
| `-addr`, `-token`, `-config` | Where to find gumpet, overriding the config file |

Or talk to the endpoint yourself:

```
curl -X POST http://127.0.0.1:8787/api/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{"text": "hello gopher", "title": "CI", "level": "success", "duration_sec": 10}'
```

A plain-text body works too, for when JSON is more ceremony than you want:

```
curl -X POST http://127.0.0.1:8787/api/v1/messages --data-binary 'hello gopher'
```

## HTTP API

The server listens on `127.0.0.1:8787` by default.

| Method | Path                 | Body                                     |
| ------ | -------------------- | ---------------------------------------- |
| `POST` | `/api/v1/messages`   | `{"text": "…", "title": "…", "level": "warn", "duration_sec": 10}`, or plain text |
| `GET`  | `/api/v1/messages`   | — (what has been received, newest first) |
| `GET`  | `/api/v1/config`     | —                                        |
| `PUT`  | `/api/v1/config`     | A whole or partial config, as JSON       |
| `POST` | `/api/v1/ask`        | `{"text": "…", "choices": ["Go", "Wait"], "timeout_sec": 60}` — waits, then answers `{"answered": true, "choice": "Go"}` |
| `GET`  | `/api/v1/healthz`    | —                                        |
| `GET`  | `/`                  | the settings page                        |
| `GET`  | `/messages`          | the messages page                        |

Only `text` is required. `duration_sec` overrides `message.duration_sec` for
that one message. Set `server.token` in the config to require a token, sent as
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
| `behavior.jump` | `true` | Hop now and then while on the floor |
| `behavior.react` | `true` | Move when a message arrives, by how serious it is |
| `behavior.quiet.from` / `.to` | *(empty)* | A time of day, `HH:MM`, when the pet says nothing |
| `behavior.chatter.enabled` | `false` | Let the pet talk to itself between messages |
| `pet.source` | `gopher` | A bundled pet by name, or your own artwork — see below |
| `pet.scale` | 1.0 | Relative to how the artwork is meant to be drawn |
| `pet.smooth` | `auto` | `auto`, `on` or `off` — how the artwork is enlarged |
| `message.max_visible` | 3 | How many balloons may be on screen at once |
| `message.max_width` | 640 | How wide the balloon may grow before the text wraps |
| `message.text_scale` | 1.5 | Message text size, relative to a 12px base |
| `message.type_speed` | 45 | Characters a second as a message appears; 0 shows it all at once |
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
restart does not always put it back in the same corner. With `behavior.jump`
on, a pet with a floor to stand on — `none` or `horizontal` — hops every so
often.

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

`behavior.quiet` sets a time of day when the pet says nothing at all. Messages
still arrive and are kept on the messages page; when the quiet ends, the pet
says once how many came in. A `to` earlier than `from` crosses midnight.

## Messages

Messages go into a queue and the pet works through it oldest first. When
several arrive at once it says several at once: up to `message.max_visible`
balloons pile up above it, each offset to one side, so a burst looks like a
crowd talking rather than a tidy queue. The oldest of them is the one at the
bottom, with the tail.

A message's level colours its balloon — green for `success`, amber for `warn`,
red for `error` — and, with `behavior.react` on, moves the pet: it jumps at a
success, hops at a warning and shivers at an error. Ordinary messages leave it
be.

Clicking a balloon takes it down, so a message that has been read does not have
to be waited out. Whatever is next in the queue moves up into its place. The pet
holds still while the cursor is over a balloon, so it stays where you aimed.
URLs in a message are drawn as links, and clicking one opens it in your browser
unless `message.open_links` is off.

Everything gumpet is sent is listed at
[http://127.0.0.1:8787/messages](http://127.0.0.1:8787/messages), newest first,
with whether the pet has said it yet. That covers messages still waiting their
turn, and ones that arrived while the pet was too busy to take them. The page
has a search box that narrows the list as you type, matches highlighted, and a
time range to go with it — the last 10 minutes through to the last 3 days.

The list is kept in memory, so it starts empty every time gumpet runs.
`history.max` and `history.hours` decide how much of it is kept; whichever
limit bites first wins.

## Talking to itself

Turn on `behavior.chatter.enabled` and the pet mutters something every
`interval_sec` or so when nobody has sent it anything — from the list gumpet
ships with, or from a file of your own sayings in `chatter.source`.

Or give it RSS or Atom feeds in `chatter.feeds`, and it reads out headlines
instead, with the feed's name above and the link clickable:

```yaml
behavior:
  chatter:
    enabled: true
    feeds:
      - name: Go Blog
        url: https://go.dev/blog/feed.atom
```

Feeds are the only thing that makes gumpet connect out on its own; with none
set, which is the default, it makes no outgoing requests at all. Only the
headlines are fetched, never the articles.

## Agents and questions

[contrib/claude-code](../contrib/claude-code) has a hook that points
[Claude Code](https://claude.com/claude-code) at your pet: the gopher asks you
Claude's questions and tells you when it has finished working, so you can leave
the terminal and still know when you are needed.

It can also answer for you. As a `PermissionRequest` hook,
`gumpetctl hook permission` puts Claude's request to run a command or edit a
file on the pet with **Allow** and **Deny** buttons, and the one you press is
Claude's answer. [Codex](../contrib/codex) uses the same hook format, so the
same command works there. If nobody answers, or gumpet is not running, the agent
asks in its terminal as it always did.

`gumpetctl ask` puts any question to the pet the same way, for your own
scripts:

```
gumpetctl ask -choices "Deploy,Wait" "main is green. Deploy?"
```

It prints the choice, or exits 3 if nobody answered.

## The menu

Clicking the pet opens a short menu with:

- which gumpet this is
- **Messages…** — opens the messages page in your browser
- **Settings…** — opens the settings page
- **Pet** — cycles through the bundled pets
- **Walk** — cycles through the four roaming styles
- **Restart** — quits and starts again, for when something on screen has gone
  wrong and a fresh start is quicker than working out what
- **Quit**

It holds only what is worth switching while looking at the pet. Everything
else is on the settings page, which keeps what people change most at the top
and the rest under **Advanced**. Changes made in either place are written to
the config file and show up in the other.

The menu speaks the language of the `language` setting — `auto` follows the
system.

The pet stops walking while the cursor is on it or on one of its balloons, so
neither is a moving target however fast it is going. The menu closes when you
pick something from it, when you click elsewhere on the pet, or a few seconds
after the cursor leaves it.

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

A name wins over a file of the same spelling. Write `./rose` if you mean the
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

## Updating

A copy installed from the releases page can update itself: press **Check for
updates** at the bottom of the settings page. If a newer release is out, one
more click downloads it, checks it against the release's `SHA256SUMS`, puts it
in place of the running `gumpet` (and of `gumpetctl`, if it sits in the same
folder) and restarts.

gumpet only asks GitHub when you press it; it never checks on its own. A build from source — `make build` or `go install` — reports whether a
newer release exists but leaves updating it to git, and a copy in a folder you
cannot write to says so before downloading anything.

The checksum proves the download arrived whole, not who made it: it comes from
the same release as the archive.

## Notes

- The window is transparent, undecorated, always on top, and hides itself from
  the Windows taskbar. It is only as big as the pet and whatever is above it,
  and moves with the pet rather than the pet moving inside it. In `on-message`
  mode it turns click-through until there is something to show, so an invisible
  pet never swallows a click.
- The settings page is served by gumpet itself and loads nothing from the
  network, so it works offline.
