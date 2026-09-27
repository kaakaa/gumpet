# Codex permission requests

When [Codex](https://developers.openai.com/codex) asks whether it may run a
command or apply a patch, gumpet can put the question on the pet with **Allow**
and **Deny** buttons. Pressing one answers Codex directly, so you can keep
working in another window.

Codex and Claude Code share the same `PermissionRequest` hook format, so this is
the same `gumpetctl hook permission` that [contrib/claude-code](../claude-code)
uses.

## Install

Add it to `~/.codex/hooks.json` (or `.codex/hooks.json` inside a project):

```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "gumpetctl hook permission -timeout 60",
            "timeout": 90,
            "statusMessage": "Asking on the desktop pet"
          }
        ]
      }
    ]
  }
}
```

Leaving out `matcher` asks about every tool Codex requests permission for:
shell commands, patches, and MCP tools. Set `"matcher": "Bash"` to be asked
only about commands.

Codex will not run a new hook until you trust it: open `/hooks` in Codex and
review it once.

`gumpetctl` has to be on the `PATH` Codex runs hooks with; give its full path
otherwise.

## What happens

The balloon's heading says who is asking and from which project —
`Codex · api` — and the text is the command, or for a patch the files it
touches.

**The pet is a shortcut, never the only way through.** If nobody presses a
button within `-timeout` seconds, or gumpet is not running, or it is in its
quiet hours, the hook prints nothing and Codex shows its usual approval prompt.
Keep the hook's own `timeout` longer than `-timeout`, or Codex stops the hook
before it can step aside.

A denied request tells Codex that you refused it from the pet. Nothing is added
to your approval rules.
