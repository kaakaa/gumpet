package lang

// japanese translates the pet's own words. The keys are the English exactly as
// the code says it; see the package documentation.
//
// The menu is narrow and read at a glance, so these are kept short, and where
// the settings page already names the same thing they use its words.
var japanese = map[string]string{
	// The menu's rows.
	"Settings…": "設定…",
	"Messages…": "メッセージ…",
	"Pet":       "ペット",
	"Walk":      "歩き方",
	"Restart":   "再起動",
	"Quit":      "終了",

	// What the rows show on the right.
	"custom":     "自分の絵",
	"none":       "動かない",
	"horizontal": "往復",
	"perimeter":  "縁を回る",
	"wander":     "歩き回る",

	// What the balloons say about themselves.
	"seen": "既読",

	// What the pet says itself.
	"While it was quiet":                       "静かにしている間",
	"{0} messages arrived while it was quiet.": "静かにしている間に {0} 件のメッセージが届きました。",
	"1 message arrived while it was quiet.":    "静かにしている間に 1 件のメッセージが届きました。",
}
