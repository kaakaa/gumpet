#!/usr/bin/env bash
#
# gumpet-notify.sh — tells a running gumpet what Claude Code is up to.
#
# Installed as a Claude Code hook; see the "hooks" block in
# ~/.claude/settings.json. Claude Code puts the event's JSON on stdin, and this
# script takes the kind of notification as its first argument:
#
#   question   Claude is asking you something (PreToolUse / AskUserQuestion)
#   done       Claude has finished its turn (Stop)
#
# A turn that finished is not necessarily a task that finished — answering a
# question and spending ten minutes rewriting a package both end at Stop. The
# "done" notification tells them apart by reading the transcript: a turn that
# used no tools was a reply, anything else was work.
#
# It never fails and never blocks. A desktop pet is not worth interrupting a
# session over, so a gumpet that is not running is simply a no-op.
#
# Environment:
#   GUMPET_NOTIFY_REPLIES=0   say nothing when a turn was only a reply
#   GUMPET_NOTIFY_DEBUG=1     record every payload, to see what an event carries

set -uo pipefail

kind="${1:-}"
payload="$(cat 2>/dev/null || true)"

# gumpet's own config is the one source of truth for where it listens and
# whether it wants a token. The file is rendered from gumpet's template, so
# these two lines always have exactly this shape.
config="${GUMPET_CONFIG:-$HOME/Library/Application Support/gumpet/config.yaml}"
addr="127.0.0.1:8787"
token=""
if [ -r "$config" ]; then
	from_config="$(sed -n 's/^  addr: "\(.*\)"$/\1/p' "$config" | head -n 1)"
	[ -n "$from_config" ] && addr="$from_config"
	token="$(sed -n 's/^  token: "\(.*\)"$/\1/p' "$config" | head -n 1)"
fi

# Claude Code does not document every field of every event, so this is the way
# to find out what one actually carries.
if [ -n "${GUMPET_NOTIFY_DEBUG:-}" ]; then
	printf '%s %s %s\n' "$(date -Iseconds)" "$kind" "$payload" \
		>>"${TMPDIR:-/tmp}/gumpet-notify-debug.log" 2>/dev/null || true
fi

# field pulls one value out of the payload, quietly, so a payload shaped
# differently than expected costs nothing.
field() {
	printf '%s' "$payload" | jq -r "$1" 2>/dev/null || true
}

# summarise_turn prints "<tool calls>\t<seconds>\t<top tools>" for the turn that
# just ended, or nothing at all if the transcript cannot be read or does not
# reach back to the start of the turn.
summarise_turn() {
	local transcript="$1"
	[ -r "$transcript" ] || return 0
	tail -n 4000 "$transcript" 2>/dev/null | jq -s -r '
		# The turn begins at the last genuine user prompt. Those carry their
		# content as a string, while a tool result carries an array.
		. as $rows
		| ([range(0; $rows | length)
		    | select($rows[.].type == "user"
		             and (($rows[.].message.content? | type) == "string"))]
		   | last) as $start
		| if $start == null then empty else
		    $rows[$start:] as $turn
		    | [$turn[] | select(.type == "assistant") | (.message.content // [])[]?
		       | select(.type == "tool_use") | .name] as $tools
		    | [$turn[] | .timestamp // empty
		       | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601] as $times
		    | [($tools | length),
		       (if ($times | length) > 1 then (($times | max) - ($times | min)) else 0 end),
		       ($tools | group_by(.) | map({name: .[0], n: length}) | sort_by(-.n)
		        | .[0:3] | map(.name + "x" + (.n | tostring)) | join(", "))]
		    | @tsv
		  end
	' 2>/dev/null || true
}

# human_time turns a count of seconds into something worth reading at a glance.
human_time() {
	local total="$1"
	if [ "$total" -lt 60 ]; then
		printf '%d秒' "$total"
	elif [ "$total" -lt 3600 ]; then
		printf '%d分%d秒' "$((total / 60))" "$((total % 60))"
	else
		printf '%d時間%d分' "$((total / 3600))" "$((total % 3600 / 60))"
	fi
}

# A turn that ends by asking something reaches Stop as well, and neither
# "finished" nor "replied" is the right thing to say about it. The question
# notification leaves a note behind that the end-of-turn one checks for.
session="$(field '.session_id // empty')"
asked_marker="${TMPDIR:-/tmp}/gumpet-asked-${session:-unknown}"
readonly asked_window=20

case "$kind" in
question)
	text="$(field '.tool_input.questions[0].question // empty')"
	[ -n "$text" ] || text="確認したいことがあります"
	# Several questions can arrive in one call; only the first is shown.
	count="$(field '.tool_input.questions | length // empty')"
	if [ "${count:-1}" -gt 1 ] 2>/dev/null; then
		text="$text（ほか $((count - 1)) 件）"
	fi
	message="質問: $text"
	seconds=30
	date +%s >"$asked_marker" 2>/dev/null || true
	;;
done)
	if [ -r "$asked_marker" ]; then
		asked_at="$(cat "$asked_marker" 2>/dev/null || true)"
		rm -f "$asked_marker" 2>/dev/null || true
		if [ -n "$asked_at" ] && [ $(($(date +%s) - asked_at)) -lt "$asked_window" ] 2>/dev/null; then
			# The question balloon already says what this one would.
			exit 0
		fi
	fi

	summary="$(summarise_turn "$(field '.transcript_path // empty')")"
	tools="$(printf '%s' "$summary" | cut -f1)"
	elapsed="$(printf '%s' "$summary" | cut -f2)"
	top="$(printf '%s' "$summary" | cut -f3)"

	if [ "${tools:-x}" = "0" ]; then
		# Nothing was done but talking.
		[ "${GUMPET_NOTIFY_REPLIES:-1}" = "0" ] && exit 0
		message="返答: 応答が終わりました"
		seconds=8
	elif [ -n "${tools:-}" ]; then
		message="完了: $(human_time "${elapsed:-0}")・${top:-作業しました}"
		seconds=15
	else
		# No usable transcript, so there is no telling which it was.
		message="完了: 作業が終わりました"
		seconds=12
	fi
	;;
*)
	exit 0
	;;
esac

# Which project, for when more than one session is running. No emoji anywhere:
# gumpet draws with a single font and does not fall back to an emoji one, so
# they would come out as empty boxes.
cwd="$(field '.cwd // empty')"
[ -n "$cwd" ] && message="$message"$'\n'"[$(basename "$cwd")]"

body="$(jq -n --arg text "$message" --argjson seconds "$seconds" \
	'{text: $text, duration_sec: $seconds}' 2>/dev/null)"
[ -n "$body" ] || exit 0

curl_args=(--silent --max-time 3 --output /dev/null
	--request POST "http://$addr/api/v1/messages"
	--header 'Content-Type: application/json')
[ -n "$token" ] && curl_args+=(--header "X-Gumpet-Token: $token")

curl "${curl_args[@]}" --data-binary "$body" 2>/dev/null || true
exit 0
