# Claude Code notifications

Have [Claude Code](https://claude.com/claude-code) send its news to your pet:
the gopher asks you Claude's questions and tells you when it has finished
working, so you can leave the terminal and still know when you are needed.

`gumpet-notify.sh` is a Claude Code hook. It reads the event Claude Code hands
it, works out what to say, and posts it to a running gumpet.

## Install

```
mkdir -p ~/.claude/hooks
cp gumpet-notify.sh ~/.claude/hooks/
chmod +x ~/.claude/hooks/gumpet-notify.sh
```

Then add the hooks to `~/.claude/settings.json`, merging with whatever is
already there rather than replacing the file:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [
          { "type": "command", "command": "\"$HOME/.claude/hooks/gumpet-notify.sh\" question", "timeout": 5 }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          { "type": "command", "command": "\"$HOME/.claude/hooks/gumpet-notify.sh\" done", "timeout": 5 }
        ]
      }
    ]
  }
}
```

Put it in `.claude/settings.json` inside a project instead if you only want it
there. `/hooks` in Claude Code reviews and disables what is installed.

The script needs `jq` and `curl`.

## What you get

| When | The gopher says |
| ---- | --------------- |
| Claude asks you something | `質問: <the question>` |
| Claude finishes working | `完了: 2分43秒・Editx5, Bashx12` |
| Claude only answered | `返答: 応答が終わりました` |

Each carries the project directory on a second line, so several sessions at
once stay apart.

**Finishing a turn is not the same as finishing a task** — answering a question
and spending ten minutes rewriting a package both end at the same `Stop` event.
The script tells them apart by reading the session transcript back to your last
message and counting the tool calls in between: none means it was a reply,
anything else means it was work, reported with how long it took and which tools
it leant on. A turn that ends by asking you something says only that, since the
question is the part worth reading.

## Settings

Set these in the hook's environment:

| | |
| ------------------------ | --- |
| `GUMPET_NOTIFY_REPLIES=0` | Say nothing about turns that were only a reply |
| `GUMPET_NOTIFY_DEBUG=1`   | Record every payload to `$TMPDIR/gumpet-notify-debug.log` |

The listen address and token come from gumpet's own config file, so changing
either needs nothing here. `$GUMPET_CONFIG` overrides where that is read from.

A gumpet that is not running is simply a no-op: the script always exits 0 and
gives up after three seconds, because a desktop pet is not worth interrupting a
coding session over.

## Other events

Claude Code fires hooks for a good deal more than this — `SessionStart`,
`PreCompact`, `PostToolUse` on a matcher of your choosing. The script takes the
kind of notification as its first argument, so adding one is a case of another
branch in the `case` statement and another entry in the settings file.
