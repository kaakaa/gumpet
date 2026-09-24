package lang

// japanese translates the pet's own words. The keys are the English exactly as
// the code says it; see the package documentation.
//
// The menu is narrow and read at a glance, so these are kept short, and where
// the settings page already names the same thing they use its words.
var japanese = map[string]string{
	// The menu's rows.
	"Say something":         "何か言わせる",
	"Settings…":             "設定…",
	"Messages…":             "メッセージ…",
	"Updates…":              "アップデート…",
	"Pet":                   "ペット",
	"Walk":                  "歩き方",
	"Roam the whole screen": "画面全体を歩き回る",
	"Always on top":         "常に最前面",
	"When idle":             "暇なとき",
	"Quit":                  "終了",

	// What the rows show on the right.
	"test":       "テスト",
	"browser":    "ブラウザ",
	"custom":     "自分の絵",
	"on":         "オン",
	"off":        "オフ",
	"none":       "動かない",
	"horizontal": "往復",
	"perimeter":  "縁を回る",
	"wander":     "歩き回る",
	"stay":       "そのまま",
	"fade":       "薄くなる",
	"hide":       "隠れる",

	// What the pet says itself.
	"Hello!":             "こんにちは！",
	"While it was quiet": "静かにしている間",
	"{0} messages arrived while it was quiet.": "静かにしている間に {0} 件のメッセージが届きました。",
	"1 message arrived while it was quiet.":    "静かにしている間に 1 件のメッセージが届きました。",
}
