(function () {
  "use strict";

  // The English on this page is the source text, not a key: every translatable
  // node carries data-i18n and keeps its English inside it. So the page still
  // reads before any of this runs, a missing translation shows English rather
  // than a key, and there is only ever one copy of the English to edit.
  //
  // STRINGS therefore holds translations only. A test walks this file and
  // fails if the page says something the dictionary has not been told about,
  // or if the dictionary still holds a sentence the page no longer says.
  var LANG_KEY = "gumpet-lang";
  var STRINGS = {
    ja: {
      "settings": "の設定",
      "Messages →": "メッセージ →",
      "This gumpet needs a token.": "この gumpet はトークンを求めています。",
      "Unlock": "解除",

      "Monitor": "ディスプレイ",
      "The display the pet lives on. It stays on that one.": "ペットが住むディスプレイ。ここから出ていきません。",
      "Roam the whole screen": "画面全体を歩き回る",
      "gumpet's window is only as big as the pet and follows it around, so letting it roam wider costs no screen space.": "gumpet のウィンドウはペットと同じ大きさしかなく、ペットについて回ります。広く歩かせても画面が狭くなるわけではありません。",
      "Corner": "寄せる場所",
      "Bottom right": "右下",
      "Bottom left": "左下",
      "Top right": "右上",
      "Top left": "左上",
      "Center": "中央",
      "Custom position": "座標で指定",
      "Which part of the monitor the pet keeps to.": "ディスプレイのどのあたりにペットを置いておくか。",
      "Size": "大きさ",
      "The area the pet can walk around in, in pixels.": "ペットが歩き回れる範囲。ピクセル。",
      "Margin": "余白",
      "Distance from the corner. Raise the second one to clear the macOS Dock.": "隅からの距離。macOS の Dock を避けるには2つ目を大きくします。",
      "Monitor coordinates, used only when the corner is \"Custom position\".": "ディスプレイ上の座標。寄せる場所が「座標で指定」のときだけ使われます。",

      "Pet": "ペット",
      "Movement": "動き",
      "On screen": "表示",
      "Talking to itself": "独り言",
      "Advanced": "詳細設定",
      "Balloons": "吹き出し",
      "Drawing": "描き方",
      "The pets gumpet ships with, or artwork of your own.": "gumpet に同梱されているペットか、自分で用意した絵。",
      "Artwork": "絵",
      "Path to a PNG, JPEG, animated GIF, or a directory of frames played in file-name order.": "PNG・JPEG・アニメーション GIF のパス。または、ファイル名順に再生するコマを入れたディレクトリ。",
      "Enlarging": "拡大のしかた",
      "Let the artwork decide": "絵にまかせる",
      "Smooth": "なめらかにする",
      "Keep the edges hard": "輪郭を残す",
      "Pixel art wants hard edges — smoothing only blurs the squares it is drawn from. The bundled pets already know which they are.": "ピクセルアートに要るのは硬い輪郭で — なめらかにしても、描かれている四角が滲むだけです。同梱のペットは自分がどちらなのかを知っています。",
      "Scale": "倍率",
      "The built-in gopher is 200 × 200 pixels at 1.0.": "同梱の gopher は 1.0 で 200 × 200 ピクセルです。",
      "Frame rate": "コマ数",
      "Ignored for animated GIFs, which carry their own timing.": "アニメーション GIF は自分でタイミングを持っているので、この値は使われません。",
      "Mirror when walking right": "右へ歩くときは反転する",
      "Turn this off if your artwork should never be flipped.": "絵を決して反転させたくないときは off に。",

      "When to show the pet": "ペットを出しておくとき",
      "Always on screen": "常に出しておく",
      "On screen, but faded when idle": "出しておくが、暇なときは薄くする",
      "Only while a message is up": "メッセージが出ている間だけ",
      "Faded to": "薄さ",
      "How solid the pet is while it has nothing to say: 1 is fully opaque, 0.35 is a faint outline. It goes back to full whenever it speaks or you open its menu.": "言うことが何も無い間のペットの濃さ。1 で完全に不透明、0.35 でうっすら輪郭が見える程度。喋るときと、メニューを開いたときは元の濃さに戻ります。",
      "How it gets around": "動きかた",
      "Stands still": "動かない",
      "Walks back and forth": "左右に往復する",
      "Walks round the edges": "画面の縁を回る",
      "Wanders anywhere": "どこでも歩き回る",
      "Walking speed": "歩く速さ",
      "Pixels per second.": "1秒あたりのピクセル。",
      "Hop now and then": "ときどき跳ねる",
      "React to messages": "メッセージに反応する",
      "The pet shivers at an error, hops at a warning and jumps at a success, so it is noticed before the balloon is read. Ordinary messages and its own remarks leave it be.": "エラーでは震え、警告では小さく跳ね、成功では大きく跳ねます。吹き出しを読む前に気づけます。ふつうのメッセージや独り言では動きません。",
      "Only applies where there is a floor to leave: “Stand still” and “Back and forth”.": "跳ねるには離れる床が要るので、「動かない」と「左右に往復する」のときだけ効きます。",
      "Talk to itself": "独り言を言う",
      "Say something now and then when no message has arrived, so the pet is doing something between notifications. Nothing is fetched over the network unless there are feeds below. Ignored while the mode above is “Hide until a message”, since a remark would only summon the pet back.": "メッセージが来ない間もときどき何か言うようにして、通知と通知の間にもペットが何かしているようにします。下にフィードがなければ、ネットワークからは何も取ってきません。上のモードが「メッセージが出ている間だけ」のときは無視されます — 独り言のためにペットを呼び戻すことになるので。",
      "How often": "間隔",
      "Seconds between remarks, give or take 30%.": "独り言と独り言の間の秒数。前後30%ほどばらつきます。",
      "Sayings file": "独り言のファイル",
      "One saying per line; blank lines and lines starting with # are skipped. Leave empty for the list gumpet ships with.": "1行に1つ。空行と # で始まる行は読み飛ばします。空にすると gumpet に同梱されている一覧を使います。",
      "Or feeds": "またはフィード",
      "One feed per line, as <code>name URL</code> — the URL is the last thing on the line, and whatever comes before it is the name that appears above the headline. A line with just a URL gets no name. <strong>This is the only part of gumpet that connects out to anywhere.</strong> Empty means it makes no outgoing requests at all. Only http and https, and only the headlines — never the articles.": "1行に1つ、<code>名前 URL</code> の形で — URL は行の最後に置き、その手前にあるものが見出しの上に出る名前になります。URL だけの行には名前が付きません。<strong>gumpet が外に接続するのはここだけです。</strong>空なら、外向きの通信は一切しません。http と https だけ、しかも見出しだけで — 記事そのものは取りに行きません。",
      "Ignore older than": "無視する古さ",
      "Days. A podcast's whole archive is a perfectly good feed, and without this the pet mostly reads out episodes from years ago. 0 keeps everything. Items a feed did not date are always kept.": "日数。ポッドキャストの全アーカイブも立派なフィードで、これが無いとペットは何年も前の回ばかり読み上げます。0 ならすべて残ります。日付の無い項目は常に残ります。",
      "Re-read the feeds": "フィードの読み直し",
      "Seconds between fetches. No reason to fetch once per remark.": "取得と取得の間の秒数。独り言1回ごとに取りに行く理由はありません。",
      "Quiet hours": "静かにする時間",
      "Messages that arrive between these times are kept on the <a href=\"/messages\">messages page</a> but not said, and the pet keeps its own remarks to itself. When the quiet ends it says once how many came in. An end earlier than the start runs overnight. Leave both empty to never be quiet.": "この時間帯に届いたメッセージは<a href=\"/messages\">メッセージのページ</a>には残りますが、ペットは口にしません。独り言も言いません。静かな時間が明けたら、何件届いたかを一度だけ言います。終わりを始まりより早くすると、夜をまたぎます。両方空なら静かにはなりません。",

      "Messages": "メッセージ",
      "Time on screen": "表示時間",
      "Seconds. A message may override this when it is sent.": "秒。送るときにメッセージごとに指定することもできます。",
      "Balloons at once": "同時に出す吹き出し",
      "Messages that arrive together are shown together, stacked above the pet, up to this many. Raise it for a noisier pet.": "同時に届いたメッセージは、ペットの上に積んでこの数まで同時に出します。にぎやかにしたければ増やしてください。",
      "Balloon width": "吹き出しの幅",
      "How wide the balloon may grow before the text wraps, in pixels.": "文字が折り返すまでに吹き出しが広がってよい幅。ピクセル。",
      "Text size": "文字の大きさ",
      "Relative to the built-in font's own 12px.": "同梱フォントの 12px を 1.0 とした倍率。",
      "Queue length": "順番待ちの数",
      "How many messages may wait their turn before the oldest are dropped.": "古いものが捨てられるまでに、何件まで順番待ちできるか。",
      "Typing speed": "喋る速さ",
      "Characters a second, so a message reads as being said rather than just appearing. 0 shows it all at once. The time a message stays up is counted from when it has finished, so a long one is not given less time to be read.": "1秒あたりの文字数。ただ現れるのではなく、喋っているように読めます。0 なら一度に全部出ます。表示時間は喋り終わってから数えるので、長いメッセージが読む時間を損することはありません。",
      "Open links when clicked": "クリックでリンクを開く",
      "URLs in a message are spotted on their own and always drawn as links. This decides whether clicking one opens it. Only http and https are ever opened — anything that can send gumpet a message can put a link in front of you.": "メッセージの中の URL はひとりでに見つけて、常にリンクとして描きます。ここで決めるのは、クリックしたときに開くかどうかだけです。開くのは http と https だけ — gumpet にメッセージを送れるものは、あなたの目の前にリンクを置けるということでもあります。",
      "Try it": "試す",
      "Send": "送信",
      "Sends a message to the pet right now, using the settings it is currently running with.": "いまペットが動いている設定のまま、メッセージを送ってみます。",

      "Font": "フォント",
      "Font file": "フォントファイル",
      "A .ttf, .otf or .ttc to draw messages and the menu with. Leave it empty and gumpet looks for one on this machine — Hiragino on macOS, Yu Gothic on Windows.": "メッセージとメニューを描くための .ttf・.otf・.ttc。空にすると、gumpet がこの端末から探します — macOS ならヒラギノ、Windows なら游ゴシック。",
      "Look for a system font": "システムのフォントを探す",
      "Turn this off to use gumpet's own bitmap font instead. It is always available, but goes blocky when enlarged.": "off にすると、代わりに gumpet 自身のビットマップフォントを使います。必ず使えますが、拡大すると粗くなります。",

      "Message history": "メッセージの履歴",
      "Keep": "残す件数",
      "How many messages the <a href=\"/messages\">messages page</a> remembers, newest first.": "<a href=\"/messages\">メッセージのページ</a>が新しい順に覚えておく件数。",
      "For": "残す時間",
      "Hours. 0 keeps them until the limit above is reached.": "時間。0 なら、上の件数に達するまで残ります。",

      "Window": "ウィンドウ",
      "Pet's language": "ペットの言語",
      "Automatic": "自動",
      "The language of the pet's menu and of what it says itself. Automatic follows the system. Messages it is sent are shown as they were sent. This page's own language is the one chosen at the top.": "ペットのメニューと、ペットが自分から言うことの言語。「自動」ならシステムの言語に合わせます。送られてきたメッセージは送られたとおりに出します。この画面自体の言語は、上で選んだものです。",
      "Always on top": "常に最前面",
      "Click through": "クリックを透かす",
      "Let clicks reach whatever is behind the pet, so it never steals one. Turning this on also means clicking the pet no longer opens its menu.": "ペットの後ろにあるものへクリックを届かせ、ペットがクリックを横取りしないようにします。on にすると、ペットをクリックしてもメニューは開かなくなります。",
      "Hide from the taskbar": "タスクバーに出さない",
      "restart": "要再起動",
      "Windows only.": "Windows のみ。",

      "Server": "サーバー",
      "Listen address": "待ち受けアドレス",
      "Keep this on 127.0.0.1 unless you want the network to reach your desktop.": "ネットワークからデスクトップに届いてほしいのでなければ、127.0.0.1 のままにしてください。",
      "Token": "トークン",
      "When set, every API request must present it.": "設定すると、すべての API 呼び出しがこれを示す必要があります。",

      "Check for updates": "アップデートを確認",
      "Asking GitHub…": "GitHub に問い合わせています…",
      "{0} is out.": "{0} が出ています。",
      "Update to {0} and restart": "{0} に更新して再起動",
      "What changed in {0}": "{0} の変更点",
      "{0} is out, but this copy was built from source. Update it with git pull and make.": "{0} が出ていますが、これはソースからビルドされたものです。git pull と make で更新してください。",
      "{0} is out, but no release is built for this machine. Build it from source.": "{0} が出ていますが、この環境向けのリリースはありません。ソースからビルドしてください。",
      "This copy has no version, so it cannot tell whether it is out of date. The latest release is {0}.": "この gumpet には版が付いていないので、古いかどうか判断できません。最新のリリースは {0} です。",
      "This is the latest version.": "最新の版です。",
      "Could not ask GitHub: {0}": "GitHub に問い合わせられませんでした: {0}",
      "Downloading {0}…": "{0} をダウンロードしています…",
      "Installed {0}. gumpet is restarting…": "{0} を入れました。gumpet を再起動しています…",
      "Could not update: {0}": "更新できませんでした: {0}",
      "gumpet has not come back after a minute. It may need starting by hand.": "1分たっても gumpet が戻ってきません。手で起動し直す必要があるかもしれません。",

      "Save": "保存",
      "Revert": "元に戻す",

      "(the built-in gopher)": "（同梱の gopher）",
      "the bundled list": "同梱の一覧",
      "(a font from this machine)": "（この端末にあるフォント）",
      "(no token required)": "（トークンは不要）",

      "Loading…": "読み込み中…",
      "Unsaved changes": "未保存の変更",
      "Saved": "保存済み",
      "Saving…": "保存中…",
      "Locked": "ロック中",
      "Error": "エラー",
      "Not saved": "保存できませんでした",
      "Could not load settings": "設定を読み込めませんでした",
      "Settings saved. The pet picked them up straight away — except the listen address and the taskbar setting, which wait for a restart.": "保存しました。ペットはすぐに拾いました — 待ち受けアドレスとタスクバーの設定だけは、次に起動したときからです。",
      "Sent. Look at your pet.": "送りました。ペットを見てください。",
      "Custom…": "自分の絵…",
      "{0}. (not attached)": "{0}. （つながっていません）",
      "Takes effect the next time gumpet starts": "次に gumpet を起動したときから効きます"
    }
  };

  var TOKEN_KEY = "gumpet-token";
  var fields = Array.prototype.slice.call(document.querySelectorAll("[data-path]"));
  var loaded = null;   // the config as the server last reported it
  var token = "";
  try { token = sessionStorage.getItem(TOKEN_KEY) || ""; } catch (e) { token = ""; }

  var $ = function (id) { return document.getElementById(id); };

  // Whatever the page says right now is read once, before anything replaces it.
  var translatable = Array.prototype.map.call(
    document.querySelectorAll("[data-i18n]"), function (el) {
      return { el: el, en: collapse(el.innerHTML) };
    });
  var placeholders = Array.prototype.map.call(
    document.querySelectorAll("[data-i18n-placeholder]"), function (el) {
      return { el: el, en: el.placeholder };
    });
  var lang = pickLang();

  function collapse(s) { return s.replace(/\s+/g, " ").trim(); }

  // pickLang takes the remembered choice, then whatever the browser asks for,
  // then English. It is kept per browser rather than in the config because it
  // is about whoever is reading this page and not about the pet: two people at
  // two machines can want different answers from one gumpet.
  function pickLang() {
    var saved = "";
    try { saved = localStorage.getItem(LANG_KEY) || ""; } catch (e) { saved = ""; }
    if (saved === "en" || STRINGS[saved]) { return saved; }
    var want = navigator.languages || [navigator.language || ""];
    for (var i = 0; i < want.length; i++) {
      var code = String(want[i]).toLowerCase().split("-")[0];
      if (STRINGS[code]) { return code; }
    }
    return "en";
  }

  function t(en) {
    var dict = STRINGS[lang];
    return (dict && dict[en]) || en;
  }

  // dyn is t for text this script creates rather than finds, so that it can be
  // said again in another language without rebuilding whatever holds it.
  function dyn(el, en, arg) {
    el.setAttribute("data-i18n-dyn", en);
    // An element said again with no argument must not keep the last one.
    if (arg !== undefined) { el.setAttribute("data-i18n-arg", arg); } else { el.removeAttribute("data-i18n-arg"); }
    redyn(el);
  }

  function redyn(el) {
    el.textContent = t(el.getAttribute("data-i18n-dyn"))
      .replace("{0}", el.getAttribute("data-i18n-arg") || "");
  }

  // applyLang says everything again in the current language. It never touches
  // a field's value: changing the language is not a reason to lose what
  // somebody was in the middle of typing.
  function applyLang() {
    document.documentElement.lang = lang;
    // These translations are constants in this file — nothing a sender, a feed
    // or the config can reach — which is what makes it safe to put them back
    // as HTML, and that is what keeps the hints containing a link or a <code>
    // working.
    translatable.forEach(function (item) { item.el.innerHTML = t(item.en); });
    placeholders.forEach(function (item) { item.el.placeholder = t(item.en); });
    document.querySelectorAll("[data-i18n-dyn]").forEach(redyn);
    $("lang").value = lang;
    status(statusKey);
    if (lastNote) { note(lastNote.kind, lastNote.key, lastNote.extra); }
    if (restartRequired) { markRestart(); }
  }

  function get(obj, path) {
    return path.split(".").reduce(function (o, k) { return o == null ? undefined : o[k]; }, obj);
  }

  function set(obj, path, value) {
    var keys = path.split(".");
    var last = keys.pop();
    keys.reduce(function (o, k) { return o[k]; }, obj)[last] = value;
  }

  function request(method, url, body) {
    var headers = {};
    if (token) { headers["X-Gumpet-Token"] = token; }
    if (body !== undefined) { headers["Content-Type"] = "application/json"; }
    return fetch(url, {
      method: method,
      headers: headers,
      body: body === undefined ? undefined : JSON.stringify(body)
    }).then(function (res) {
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

  var lastNote = null;

  function note(kind, key, extra) {
    lastNote = { kind: kind, key: key, extra: extra || "" };
    var el = $("banner");
    el.className = "note " + kind;
    el.textContent = t(key) + lastNote.extra;
    el.hidden = false;
  }

  function clearNote() { lastNote = null; $("banner").hidden = true; }

  var statusKey = "Loading…";

  function status(key) { statusKey = key; $("status").textContent = t(key); }

  // The restart tags only earn a title once the server has said which settings
  // wait for one, and it has to be said again when the language changes.
  var restartRequired = false;

  function markRestart() {
    document.querySelectorAll("[data-restart]").forEach(function (el) {
      el.title = t("Takes effect the next time gumpet starts");
    });
  }

  // The monitors are whatever the machine reported when gumpet started, so the
  // choice is a list of displays by name rather than a number to be guessed at.
  function fillMonitors(monitors, chosen) {
    var select = $("display");
    select.textContent = "";

    (monitors || []).forEach(function (m) {
      var option = document.createElement("option");
      option.value = m.number;
      option.textContent = m.number + ". " + m.name + " (" + m.width + "\u00d7" + m.height + ")";
      select.appendChild(option);
    });

    // A setting naming a monitor that is no longer attached still has to show
    // as itself, or saving anything else would quietly move the pet.
    if (!Array.prototype.some.call(select.options, function (o) {
      return parseInt(o.value, 10) === chosen;
    })) {
      var missing = document.createElement("option");
      missing.value = chosen;
      dyn(missing, "{0}. (not attached)", chosen);
      select.appendChild(missing);
    }
  }

  // feedLines renders the stored list as one feed per line. The URL goes last
  // because a name may contain spaces and a URL may not, which makes the last
  // field the only one that can be picked out without a separator to argue
  // about.
  function feedLines(feeds) {
    return (feeds || []).map(function (f) {
      return f.name ? f.name + " " + f.url : f.url;
    }).join("\n");
  }

  // parseFeeds reads that back. Anything it cannot make sense of is sent to
  // the server as written, so the config's own validation is what rejects it
  // and the page does not have to hold a second opinion about what a URL is.
  function parseFeeds(text) {
    return text.split("\n").map(function (line) {
      return line.trim();
    }).filter(function (line) {
      return line !== "";
    }).map(function (line) {
      var at = line.lastIndexOf(" ");
      if (at < 0) { return { name: "", url: line }; }
      return { name: line.slice(0, at).trim(), url: line.slice(at + 1).trim() };
    });
  }

  // fillPets offers the bundled pets plus "Custom", which reveals the path
  // field. The path field stays the one source of truth: the picker only
  // writes into it, so there is never a second opinion about which pet is on.
  function fillPets(pets) {
    var sel = $("pet-pick");
    sel.textContent = "";
    (pets || []).forEach(function (p) {
      var o = document.createElement("option");
      o.value = p.name;
      o.textContent = p.label;
      sel.appendChild(o);
    });
    var custom = document.createElement("option");
    custom.value = "\u0000custom";
    dyn(custom, "Custom…");
    sel.appendChild(custom);
  }

  // syncPet points the picker at whatever the path field holds.
  function syncPet() {
    var sel = $("pet-pick");
    var value = $("source").value;
    var names = Array.prototype.map.call(sel.options, function (o) { return o.value; });

    if (value === "" && names.length > 0) { value = names[0]; }
    sel.value = names.indexOf(value) >= 0 ? value : "\u0000custom";
    $("source-row").hidden = sel.value !== "\u0000custom";
  }

  function fill(cfg) {
    fields.forEach(function (el) {
      var value = get(cfg, el.dataset.path);
      if (el.dataset.kind === "feeds") {
        el.value = feedLines(value);
      } else if (el.type === "checkbox") {
        el.checked = !!value;
      } else {
        el.value = value;
      }
    });
    syncPet();
    syncVisible();
  }

  // collect returns the loaded config with every field's current value written
  // over it, so keys the page does not show are carried through untouched.
  function collect() {
    var cfg = JSON.parse(JSON.stringify(loaded));
    fields.forEach(function (el) {
      var value;
      if (el.dataset.kind === "feeds") {
        value = parseFeeds(el.value);
      } else if (el.type === "checkbox") {
        value = el.checked;
      } else if (el.dataset.kind === "int") {
        value = parseInt(el.value, 10);
        if (isNaN(value)) { value = 0; }
      } else if (el.dataset.kind === "float") {
        value = parseFloat(el.value);
        if (isNaN(value)) { value = 0; }
      } else {
        value = el.value;
      }
      set(cfg, el.dataset.path, value);
    });
    return cfg;
  }

  // syncVisible hides whatever the current choices make irrelevant: the
  // corner settings while the pet roams the whole screen, the x/y boxes
  // unless that corner is "custom", the fade level unless the pet fades, and
  // the details of talking to itself unless it does. Hidden rather than greyed
  // out: a row that cannot matter is one less row to read. Its value is still
  // there, and is saved as it was.
  function syncVisible() {
    var fullscreen = $("fullscreen").checked;
    var custom = $("anchor").value === "custom";
    rowsOf("[data-corner-only]", fullscreen);
    rowsOf("[data-custom-only]", fullscreen || !custom);
    rowsOf("[data-faded-only]", $("mode").value !== "faded");
    rowsOf("[data-chatter-only]", !$("chatter").checked);
  }

  function rowsOf(selector, hide) {
    document.querySelectorAll(selector).forEach(function (el) {
      el.closest(".row").hidden = hide;
    });
  }


  // changes returns only the settings whose value differs from the ones this
  // page loaded. Sending the whole config instead would mean a page left open
  // in a tab could quietly undo something changed from the pet's own menu in
  // the meantime; the server merges a partial body into whatever is current.
  function changes() {
    return diff(loaded, collect());
  }

  function diff(was, now) {
    var out = {};
    Object.keys(now).forEach(function (key) {
      var before = was ? was[key] : undefined;
      var after = now[key];
      // A list is sent whole or not at all. Recursing into one would send
      // {"0": …} instead of an array, and would never notice a row that was
      // deleted, since only the keys still present are walked.
      if (Array.isArray(after)) {
        if (JSON.stringify(before) !== JSON.stringify(after)) { out[key] = after; }
      } else if (after !== null && typeof after === "object") {
        var nested = diff(before, after);
        if (Object.keys(nested).length > 0) { out[key] = nested; }
      } else if (before !== after) {
        out[key] = after;
      }
    });
    return out;
  }

  function dirty() {
    return loaded !== null && Object.keys(changes()).length > 0;
  }

  function refreshButtons() {
    var changed = dirty();
    $("save").disabled = !changed;
    $("revert").disabled = !changed;
    if (changed) { status("Unsaved changes"); } else { status("Saved"); }
  }

  function adopt(data) {
    loaded = data.config;
    $("path").textContent = data.path;
    fillMonitors(data.monitors, loaded.stage.display);
    fillPets(data.pets);
    $("version").textContent = data.version || "";
    // No version means the server has no updater to ask.
    $("update-check").hidden = !data.version;
    restartRequired = !!(data.restart_required && data.restart_required.length);
    if (restartRequired) { markRestart(); }
    fill(loaded);
    $("form").hidden = false;
    $("auth").hidden = true;
    refreshButtons();
  }

  function askForToken() {
    $("auth").hidden = false;
    $("form").hidden = true;
    status("Locked");
    $("token-input").focus();
  }

  // Arriving at #updates is asking — the pet's menu sends people here — so the
  // check runs straight away. Once: the page reloads itself here after an
  // update, and should then only say it is up to date, not loop.
  var checkedOnArrival = false;

  function load() {
    request("GET", "api/v1/config").then(function (data) {
      adopt(data);
      if (location.hash === "#updates" && !checkedOnArrival) {
        checkedOnArrival = true;
        checkUpdates();
      }
    }).catch(function (err) {
      if (err.unauthorized) { askForToken(); return; }
      note("err", "Could not load settings", ": " + err.message);
      status("Error");
    });
  }

  $("save").addEventListener("click", function () {
    clearNote();
    status("Saving…");
    request("PUT", "api/v1/config", changes()).then(function (data) {
      // Saving a new token means the next request needs it.
      token = data.config.server.token;
      try { sessionStorage.setItem(TOKEN_KEY, token); } catch (e) { /* private window */ }
      adopt(data);
      note("ok", "Settings saved. The pet picked them up straight away — except the listen address and the taskbar setting, which wait for a restart.");
    }).catch(function (err) {
      if (err.unauthorized) { askForToken(); return; }
      note("err", err.message);
      status("Not saved");
    });
  });

  // The update check asks the server, which asks GitHub. Nothing here runs
  // unless someone presses the button beside the title or arrives at
  // #updates, which is where the pet's menu sends them.
  var latestRelease = "";

  function checkUpdates() {
    var result = $("update-result");
    $("updates").hidden = false;
    $("update-apply").hidden = true;
    $("update-notes").hidden = true;
    $("update-check").disabled = true;
    dyn(result, "Asking GitHub…");
    request("GET", "api/v1/update").then(function (st) {
      latestRelease = st.latest;
      if (st.available && st.installable) {
        dyn(result, "{0} is out.", st.latest);
        dyn($("update-apply"), "Update to {0} and restart", st.latest);
        $("update-apply").disabled = false;
        $("update-apply").hidden = false;
        // The link is to the release's page and nowhere else.
        var notes = $("update-notes");
        if (/^https:\/\/github\.com\//.test(st.url || "")) {
          notes.href = st.url;
          dyn(notes, "What changed in {0}", st.latest);
          notes.hidden = false;
        }
      } else if (st.reason === "source-build") {
        dyn(result, "{0} is out, but this copy was built from source. Update it with git pull and make.", st.latest);
      } else if (st.reason === "no-build-for-platform") {
        dyn(result, "{0} is out, but no release is built for this machine. Build it from source.", st.latest);
      } else if (st.reason === "dev-build") {
        dyn(result, "This copy has no version, so it cannot tell whether it is out of date. The latest release is {0}.", st.latest);
      } else {
        dyn(result, "This is the latest version.");
      }
    }).catch(function (err) {
      if (err.unauthorized) { askForToken(); return; }
      dyn(result, "Could not ask GitHub: {0}", err.message);
    }).then(function () {
      $("update-check").disabled = false;
    });
  }

  // waitForRestart asks every second until the gumpet answering is the one it
  // was updated to, then reloads the page from it. The old one keeps answering
  // for a moment, and in between nothing answers at all; both just mean wait.
  function waitForRestart(want, since) {
    if (Date.now() - since > 60000) {
      dyn($("update-result"), "gumpet has not come back after a minute. It may need starting by hand.");
      return;
    }
    setTimeout(function () {
      request("GET", "api/v1/config").then(function (data) {
        if (data.version === want) { location.reload(); return; }
        waitForRestart(want, since);
      }).catch(function () {
        waitForRestart(want, since);
      });
    }, 1000);
  }

  $("update-check").addEventListener("click", checkUpdates);

  $("update-apply").addEventListener("click", function () {
    var result = $("update-result");
    $("update-apply").disabled = true;
    $("update-check").disabled = true;
    dyn(result, "Downloading {0}…", latestRelease);
    request("POST", "api/v1/update").then(function () {
      dyn(result, "Installed {0}. gumpet is restarting…", latestRelease);
      waitForRestart(latestRelease, Date.now());
    }).catch(function (err) {
      if (err.unauthorized) { askForToken(); return; }
      dyn(result, "Could not update: {0}", err.message);
      $("update-apply").disabled = false;
      $("update-check").disabled = false;
    });
  });

  $("revert").addEventListener("click", function () {
    clearNote();
    fill(loaded);
    refreshButtons();
  });

  $("token-save").addEventListener("click", function () {
    token = $("token-input").value.trim();
    try { sessionStorage.setItem(TOKEN_KEY, token); } catch (e) { /* private window */ }
    clearNote();
    load();
  });

  $("token-input").addEventListener("keydown", function (e) {
    if (e.key === "Enter") { $("token-save").click(); }
  });

  $("test-send").addEventListener("click", function () {
    var text = $("test-text").value;
    if (!text.trim()) { return; }
    clearNote();
    request("POST", "api/v1/messages", { text: text }).then(function () {
      note("ok", "Sent. Look at your pet.");
    }).catch(function (err) {
      if (err.unauthorized) { askForToken(); return; }
      note("err", err.message);
    });
  });

  fields.forEach(function (el) {
    el.addEventListener("input", refreshButtons);
    el.addEventListener("change", refreshButtons);
  });

  // Coming back to the tab is the moment to pick up anything changed from the
  // pet's menu. Reloading over unsaved edits would lose them, so this only
  // refreshes a page with nothing pending.
  window.addEventListener("focus", function () {
    if (loaded !== null && !dirty()) { load(); }
  });
  // Picking a bundled pet writes its name into the path field, which is what
  // actually gets saved. Choosing Custom empties it and shows the field, so
  // there is somewhere to type.
  $("pet-pick").addEventListener("change", function () {
    var picked = $("pet-pick").value;
    $("source").value = picked === "\u0000custom" ? "" : picked;
    $("source-row").hidden = picked !== "\u0000custom";
    refreshButtons();
    if (picked === "\u0000custom") { $("source").focus(); }
  });
  $("source").addEventListener("input", syncPet);

  $("lang").addEventListener("change", function () {
    lang = $("lang").value;
    try { localStorage.setItem(LANG_KEY, lang); } catch (e) { /* private window */ }
    applyLang();
  });

  $("anchor").addEventListener("change", syncVisible);
  $("fullscreen").addEventListener("change", syncVisible);
  $("mode").addEventListener("change", syncVisible);
  $("chatter").addEventListener("change", syncVisible);

  // The advanced section stays as each person left it. A convenience only: a
  // browser that will not remember simply opens it closed.
  try { $("advanced").open = localStorage.getItem("gumpet-advanced") === "open"; } catch (e) { /* private window */ }
  $("advanced").addEventListener("toggle", function () {
    try { localStorage.setItem("gumpet-advanced", $("advanced").open ? "open" : "closed"); } catch (e) { /* private window */ }
  });

  applyLang();
  load();
})();
