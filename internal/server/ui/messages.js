(function () {
  "use strict";

  var TOKEN_KEY = "gumpet-token";
  var REFRESH_MS = 3000;
  var token = "";
  try { token = sessionStorage.getItem(TOKEN_KEY) || ""; } catch (e) { token = ""; }

  var $ = function (id) { return document.getElementById(id); };
  var timer = null;
  // The last thing the server told us, kept so that typing in the search box
  // filters what is already here rather than waiting on a round trip.
  var latest = null;
  // Which list is showing. It lives in the address as #said so that the list
  // of remarks can be linked to and survives a reload.
  var view = location.hash === "#said" ? "remarks" : "messages";

  function request(url) {
    var headers = {};
    if (token) { headers["X-Gumpet-Token"] = token; }
    return fetch(url, { headers: headers }).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (res.status === 401) {
          var err = new Error("unauthorized");
          err.unauthorized = true;
          throw err;
        }
        if (!res.ok) { throw new Error(data.error || (res.status + " " + res.statusText)); }
        return data;
      });
    });
  }

  // ago renders a timestamp the way you actually read a log: how long ago it
  // was, with the clock time alongside it.
  function ago(iso) {
    var then = new Date(iso);
    var seconds = Math.max(0, Math.round((Date.now() - then.getTime()) / 1000));
    var rough;
    if (seconds < 10) { rough = "just now"; }
    else if (seconds < 60) { rough = seconds + "s ago"; }
    else if (seconds < 3600) { rough = Math.floor(seconds / 60) + "m ago"; }
    else if (seconds < 86400) { rough = Math.floor(seconds / 3600) + "h ago"; }
    else { rough = Math.floor(seconds / 86400) + "d ago"; }
    return rough + " · " + then.toLocaleTimeString();
  }

  // matching narrows the list to what the search box and the time range allow.
  function matching(messages) {
    var query = $("search").value.trim().toLowerCase();
    var seconds = parseInt($("range").value, 10) || 0;
    var cutoff = seconds > 0 ? Date.now() - seconds * 1000 : 0;

    return messages.filter(function (msg) {
      if (cutoff > 0 && new Date(msg.queued_at).getTime() < cutoff) { return false; }
      if (query === "") { return true; }
      // The title is part of the message as far as anyone reading the page is
      // concerned, so searching for "CI" should find the ones labelled CI.
      return msg.text.toLowerCase().indexOf(query) >= 0 ||
        (msg.title || "").toLowerCase().indexOf(query) >= 0 ||
        (msg.link || "").toLowerCase().indexOf(query) >= 0;
    });
  }

  // fillHighlighted writes text into el, wrapping each match in a <mark>.
  // Everything goes in as a text node: these strings come from whatever sent
  // the message, and must never be parsed as markup.
  function fillHighlighted(el, text, query) {
    if (query === "") {
      el.textContent = text;
      return;
    }
    var haystack = text.toLowerCase();
    var from = 0;
    for (;;) {
      var at = haystack.indexOf(query, from);
      if (at < 0) { break; }
      el.appendChild(document.createTextNode(text.slice(from, at)));
      var hit = document.createElement("mark");
      hit.textContent = text.slice(at, at + query.length);
      el.appendChild(hit);
      from = at + query.length;
    }
    el.appendChild(document.createTextNode(text.slice(from)));
  }

  // The feeds block: how each configured feed was last read, and what it
  // served. It is rebuilt only when a report changes — the page refreshes
  // every few seconds, and rebuilding would close a source someone is in the
  // middle of reading — and in between only the times are brought up to date.
  var feedsData = null;
  var feedsBuiltFrom = "";
  var feedRows = [];
  // Sources opened, by URL, so that a rebuild opens them again.
  var openSources = {};

  function size(n) {
    if (n < 1024) { return n + " B"; }
    if (n < 1024 * 1024) { return Math.round(n / 1024) + " KB"; }
    return (n / 1024 / 1024).toFixed(1) + " MB";
  }

  function describe(f) {
    if (!f.fetched_at) {
      return feedsData.reading
        ? "Not read yet."
        : "Not read: talking to itself is off in the settings, and feeds are only read while it is on.";
    }
    if (f.error) { return "Failed " + ago(f.fetched_at) + ": " + f.error; }
    return "Read " + ago(f.fetched_at) + " · " + f.found + (f.found === 1 ? " headline, " : " headlines, ") +
      f.recent + " recent enough to say · " + size(f.size);
  }

  function showSource(row, text) {
    var pre = row.li.querySelector(".source");
    if (!pre) {
      pre = document.createElement("pre");
      pre.className = "source";
      row.li.appendChild(pre);
    }
    // textContent, always: this is somebody else's document.
    pre.textContent = text;
    row.button.textContent = "Hide source";
  }

  function loadSource(row) {
    row.button.disabled = true;
    request("api/v1/feeds/source?url=" + encodeURIComponent(row.feed.url)).then(function (d) {
      openSources[row.feed.url] = true;
      showSource(row, d.text);
    }).catch(function (err) {
      showSource(row, "Could not load the source: " + err.message);
    }).then(function () {
      row.button.disabled = false;
    });
  }

  function renderFeeds() {
    var block = $("feeds");
    var feeds = (feedsData && feedsData.feeds) || [];
    block.hidden = view !== "remarks" || feeds.length === 0;
    if (block.hidden) { return; }

    var key = JSON.stringify(feedsData);
    if (key === feedsBuiltFrom) {
      feedRows.forEach(function (row) { row.when.textContent = describe(row.feed); });
      return;
    }
    feedsBuiltFrom = key;

    var list = $("feed-list");
    list.textContent = "";
    feedRows = feeds.map(function (f) {
      var li = document.createElement("li");
      var title = document.createElement("p");
      title.className = "title";
      title.textContent = f.name || "(no name)";
      li.appendChild(title);

      var url = document.createElement("span");
      url.className = "feed-url";
      url.textContent = f.url;
      li.appendChild(url);

      var when = document.createElement("span");
      when.className = "when" + (f.error ? " failed" : "");
      li.appendChild(when);

      var row = { feed: f, li: li, when: when, button: null };
      if (f.has_source) {
        var button = document.createElement("button");
        button.type = "button";
        button.textContent = "Source";
        button.addEventListener("click", function () {
          if (openSources[f.url]) {
            delete openSources[f.url];
            var pre = li.querySelector(".source");
            if (pre) { pre.remove(); }
            button.textContent = "Source";
            return;
          }
          loadSource(row);
        });
        li.appendChild(button);
        row.button = button;
      }
      when.textContent = describe(f);
      list.appendChild(li);
      // A source that was open is fetched again: the feed has been read
      // since, and the old text would not be what it serves now.
      if (openSources[f.url] && row.button) { loadSource(row); }
      return row;
    });
  }

  // linkTo is a feed's link, if it is one this page is prepared to offer. A
  // feed is somebody else's XML: a javascript: URL in one must not become
  // something to click.
  function linkTo(url) {
    if (!/^https?:\/\//i.test(url || "")) { return null; }
    var a = document.createElement("a");
    a.className = "link";
    a.href = url;
    a.target = "_blank";
    a.rel = "noopener noreferrer";
    a.textContent = url;
    return a;
  }

  function showTabs() {
    $("tab-messages").setAttribute("aria-selected", String(view === "messages"));
    $("tab-remarks").setAttribute("aria-selected", String(view === "remarks"));
  }

  function render() {
    showTabs();
    renderFeeds();
    if (latest === null) { return; }
    var remarks = view === "remarks";
    var all = latest[view] || [];
    var messages = matching(all);
    var query = $("search").value.trim().toLowerCase();

    var list = $("list");
    list.textContent = "";

    messages.forEach(function (msg) {
      var li = document.createElement("li");
      // Only the levels with a colour of their own get a class; "info" and
      // anything unrecognised are left looking ordinary.
      if (msg.level === "success" || msg.level === "warn" || msg.level === "error") {
        li.className = "level-" + msg.level;
      }

      if (msg.title) {
        var title = document.createElement("p");
        title.className = "title";
        // textContent, like everything else here: a title comes from whatever
        // sent the message and must never be parsed as markup.
        title.textContent = msg.title;
        li.appendChild(title);
      }

      var text = document.createElement("p");
      text.className = "text";
      fillHighlighted(text, msg.text, query);
      li.appendChild(text);

      if (remarks) {
        if (msg.seen) {
          li.classList.add("seen");
          var again = document.createElement("span");
          again.className = "badge again";
          again.textContent = "seen before";
          li.appendChild(again);
        }
        // The article is what a headline was for, and the balloon it came in
        // was gone in seconds.
        var link = linkTo(msg.link);
        if (link) { li.appendChild(link); }
      } else {
        // A remark is on screen the moment it is said, so "waiting" would
        // never apply and "shown" would say nothing.
        var state = msg.shown_at ? "shown" : (msg.held ? "held" : "waiting");
        var label = state;
        if (msg.choices) {
          // A question: what matters is what was answered.
          state = msg.answer ? "answered" : (msg.unanswered ? "unanswered" : "waiting");
          label = msg.answer ? msg.answer : (msg.unanswered ? "no answer" : "asking");
        }
        var badge = document.createElement("span");
        badge.className = "badge " + state;
        // textContent: an answer is one of the choices somebody sent.
        badge.textContent = label;
        li.appendChild(badge);
      }

      var when = document.createElement("span");
      when.className = "when";
      if (remarks) {
        when.textContent = "said " + ago(msg.shown_at) +
          (msg.published ? ", published " + new Date(msg.published).toLocaleString() : "");
      } else {
        when.textContent = msg.choices
          ? "asked " + ago(msg.queued_at) + " · " + msg.choices.join(" / ")
          : msg.shown_at
          ? "received " + ago(msg.queued_at) + ", shown " + ago(msg.shown_at)
          : msg.held
            ? "received " + ago(msg.queued_at) + ", during quiet hours"
            : "received " + ago(msg.queued_at);
      }
      li.appendChild(when);

      list.appendChild(li);
    });

    var filtered = messages.length !== all.length;
    $("empty").hidden = messages.length > 0;
    $("empty").textContent = filtered
      ? "Nothing matches."
      : remarks
        ? "Nothing yet. With \u201cTalk to itself\u201d on in the settings, what the pet says between messages is kept here."
        : "Nothing yet. Send something with gumpetctl hello.";
    $("filters").hidden = all.length === 0 && !filtered;

    var limits = latest.history || {};
    var one = remarks ? " remark" : " message";
    var many = remarks ? " remarks" : " messages";
    var counted = filtered
      ? messages.length + " of " + all.length + many
      : all.length + (all.length === 1 ? one : many);

    var parts = [counted];
    if (limits.max) { parts.push("keeping up to " + limits.max); }
    if (limits.hours > 0) {
      parts.push("for " + limits.hours + (limits.hours === 1 ? " hour" : " hours"));
    } else {
      parts.push("with no time limit");
    }
    $("summary").textContent = parts.join(", ") + ". Kept in memory, so a restart clears it.";
  }

  function load() {
    // The feeds' reports are extra: a failure there must not stop the list.
    var feeds = request("api/v1/feeds").catch(function () { return null; });
    return Promise.all([request("api/v1/messages"), feeds]).then(function (res) {
      var data = res[0];
      feedsData = res[1];
      $("auth").hidden = true;
      $("error").hidden = true;
      latest = data;
      render();
      start();
    }).catch(function (err) {
      stop();
      if (err.unauthorized) {
        $("auth").hidden = false;
        $("summary").textContent = "Locked.";
        $("token-input").focus();
        return;
      }
      $("error").textContent = "Could not load messages: " + err.message;
      $("error").hidden = false;
      $("summary").textContent = "";
    });
  }

  function start() {
    if (timer === null) { timer = setInterval(load, REFRESH_MS); }
  }

  function stop() {
    if (timer !== null) { clearInterval(timer); timer = null; }
  }

  // Polling a hidden tab is pure waste; pick straight back up on return.
  document.addEventListener("visibilitychange", function () {
    if (document.hidden) { stop(); } else { load(); }
  });

  $("token-save").addEventListener("click", function () {
    token = $("token-input").value.trim();
    try { sessionStorage.setItem(TOKEN_KEY, token); } catch (e) { /* private window */ }
    load();
  });
  $("token-input").addEventListener("keydown", function (e) {
    if (e.key === "Enter") { $("token-save").click(); }
  });

  // Filtering is done here rather than on the server, so it keeps up with
  // typing and the refresh below never disturbs what is in the box.
  $("search").addEventListener("input", render);
  $("range").addEventListener("change", render);

  function choose(which) {
    view = which;
    history.replaceState(null, "", which === "remarks" ? "#said" : location.pathname);
    render();
  }
  $("tab-messages").addEventListener("click", function () { choose("messages"); });
  $("tab-remarks").addEventListener("click", function () { choose("remarks"); });

  load();
})();
