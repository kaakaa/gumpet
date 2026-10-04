# Working on gumpet

A desktop pet that shows you your messages. Text arrives over HTTP, a gopher
walks over and says it.

## The constraint everything else follows from

**The pet's window cannot be opened without a display.** Not in CI, not from a
headless shell — Ebitengine needs a real window server, and the binary panics
during package initialisation without one. That is not a bug to route around;
it is the shape of the problem.

Two consequences:

- **A package that imports Ebitengine cannot be tested.** Its test binary dies
  at init, taking the whole `go test ./...` run with it. Only `internal/pet`,
  `internal/petpack` and `cmd/gumpet` import it, and none of them has tests.
- **Anything visual is unproven until a person runs it.** Say so, plainly,
  rather than implying otherwise. The pull request template has a section for
  exactly this.

So: **logic that can be tested lives in a package that does not import
Ebitengine.** That is why `internal/roam` walks the pet around, `internal/layout`
decides where the window goes, `internal/textwrap` breaks lines, and
`internal/gifseq` flattens GIF frames — all of it arithmetic, all of it tested,
none of it needing a screen. `internal/pet` is left holding the drawing calls.

When a change adds logic worth testing, move it into one of those, or make a new
one. When it genuinely is drawing, accept that it is unverifiable and say so.

### Checking something visual anyway

Render it offscreen with `golang.org/x/image/font` and `image/png`, using the
same measurements the game uses, and look at the picture. That is how the
balloon stack, the menu layout, the fade opacity and the font change were all
checked. Put such a program in a scratch directory, use it, delete it — it is a
tool, not a deliverable.

## Layout

| | |
| --- | --- |
| `cmd/gumpet` | the pet, the window, and the process that owns both |
| `cmd/gumpet/winres` | the icon compiled into `gumpet.exe`, drawn by hand |
| `cmd/gumpetctl` | sends a message or a question; opens the settings page; the agents' permission hook |
| `internal/pet` | drawing, input, the menu, the game loop — imports Ebitengine |
| `internal/petpack` | uploads artwork to the GPU — imports Ebitengine |
| `assets`, `internal/petsrc` | the bundled artwork; reading artwork off disk, and validating a path without a screen |
| `internal/icon` | the window's icon and the pages' favicon, made from the pet in use |
| `internal/roam`, `internal/layout`, `internal/react` | where the pet goes, where the window follows, and how it jumps at news |
| `internal/drag`, `internal/hover` | moving the pet with the mouse, and holding it still under the cursor |
| `internal/textwrap`, `internal/richtext` | line breaking, and the links and headings drawn differently within it |
| `internal/gifseq`, `internal/fontfile` | GIF frames, finding a font |
| `internal/config`, `internal/settings` | the config file, and the live copy both the pages and the menu write through |
| `internal/server` | the HTTP API and the two web pages |
| `internal/history`, `internal/message` | what has been received, and what is on its way |
| `internal/ask`, `internal/agenthook` | questions put to the pet, and the hook format Claude Code and Codex share |
| `internal/chatter`, `internal/feed`, `internal/quiet` | what the pet says on its own, the headlines it reads, and when it keeps quiet |
| `internal/lang` | the language the pet speaks, and its own words in it |
| `internal/display`, `internal/browser`, `internal/clipboard` | monitors, opening a link, copying a message |
| `internal/update` | replacing a release with a newer, signed one |

## Commands

```
go test ./...          # everything; must stay green
gofmt -l .             # must print nothing
go vet ./...
make build             # bin/gumpet and bin/gumpetctl
make windows           # cross-compiles, no cgo needed there
```

`go vet` and `go build` print deprecation warnings from Ebitengine's own macOS
code. They are not yours; ignore them.

## Adding a setting

A setting is not done until it exists in all four places. Miss one and it
silently does nothing, or silently gets wiped on the next save.

1. `internal/config/config.go` — the struct field, the default in `Default()`,
   and a check in `Validate()`
2. `internal/config/render.go` — the template, with a comment saying what it is
   for. The file is rendered, not marshalled, so comments survive a save
3. `internal/server/ui/settings.html` — an input with `data-path` naming it
4. `internal/config/config_test.go` — add it to the round-trip case, and to the
   rejection table if it can be set wrongly

`TestRenderRoundTrips` fails if the template and the struct disagree, which is
the point of it.

## Tests

Table-driven, named for what breaks rather than what runs. A failure message
should say what was wrong without opening the test:

```go
t.Errorf("balloon %d offset %v, want %v", i, got, want)
```

Where randomness is involved, seed it (`rand.NewPCG`) so a failure can be
reproduced. Where time is involved, inject a clock — `internal/history` does.

## Comments

Say why, not what. Assume the reader can see the code and cannot see the
reasoning, the alternative that was rejected, or the bug that made it necessary.

## Commits

Explain the change in prose: what was wrong, what it does now, and what was
weighed. One concern per commit.
