# Developing gumpet

How to build, test and release gumpet. The conventions the code follows — where
logic goes, how tests are written, what makes a setting complete — are in
[CLAUDE.md](CLAUDE.md), which is written for people as much as for agents.

## Building

Go 1.25 or later.

```
make build            # bin/gumpet and bin/gumpetctl
make windows          # cross-compile to bin/windows-amd64/, from any host
```

Ebitengine needs no cgo on Windows, so `GOOS=windows go build` works straight
from macOS or Linux. macOS needs cgo for Metal, so a macOS binary has to be
built on a Mac.

`go vet` and `go build` print deprecation warnings from Ebitengine's own macOS
code. They are not gumpet's; ignore them.

## Testing

```
go test ./...          # must stay green
gofmt -l .             # must print nothing
go vet ./...
```

The pet's window cannot be opened without a display, and a package that imports
Ebitengine panics during initialisation without one — so `internal/pet`,
`internal/petpack` and `cmd/gumpet` have no tests, and everything worth testing
lives in packages that do not import it. Anything visual stays unproven until
someone runs the pet and looks. [CLAUDE.md](CLAUDE.md) explains the split and
how to check a drawing offscreen.

The images in the README are made the same way: rendered offscreen from the
bundled artwork with the pet's own balloon geometry, colours and font, and its
own walking and reaction code. They are not screen recordings.

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
to the release, with two signatures over `SHA256SUMS`:

| File | Signed with | Read by |
| --- | --- | --- |
| `SHA256SUMS.key.sigstore.json` | the release key, from the repository's secrets | gumpet's updater, which refuses a release without it |
| `SHA256SUMS.sigstore.json` | no key: a short-lived Sigstore certificate naming the workflow and tag | people, with `cosign verify-blob` |

Both are recorded in Sigstore's public transparency log, and both are verified
by the workflow before the release is published. The updater checks only the
key signature, with nothing but the standard library: verifying the keyless one
in Go needs a Sigstore client that adds about 12MB to an 18MB binary.

### Setting up the release key

Once, before the first signed release. With
[cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
installed:

```
cosign generate-key-pair
gh secret set COSIGN_PRIVATE_KEY < cosign.key
gh secret set COSIGN_PASSWORD
mv cosign.pub internal/update/cosign.pub
```

`generate-key-pair` asks for a password and encrypts `cosign.key` with it; the
second `gh secret set` asks for the same password. Commit
`internal/update/cosign.pub` — it is compiled into gumpet, and
`TestTheEmbeddedReleaseKeyIsUsable` fails if it is not a key the updater can
use. Keep `cosign.key` and its password somewhere safe outside the repository
(`.gitignore` names it, so it cannot be committed by accident), or delete it:
the secret is the copy the workflow uses.

Changing the key later works for new installs only. Every gumpet already out
there checks for the old one and will refuse releases signed with the new one,
so their owners have to download the next release by hand. That is the point
of the key, and the reason not to change it lightly.
