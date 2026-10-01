# gumpet

[English](README.md) | **日本語**

<p align="center">
  <img src="docs/images/demo.gif" width="600" alt="画面の下を歩くゴーファーが、吹き出しでメッセージを言い、次に二つ重ねて言い、最後に Allow と Deny のボタンが付いた質問を出す">
</p>

<p align="center">
  <b>メッセージを知らせてくれるデスクトップペット。</b><br>
  HTTP でテキストを送ると、ゴーファーが歩いてきてしゃべります。
</p>

```
gumpetctl "デプロイ終わったよ"
```

`gum` は `ugm` の並べ替えで、**u**'ve **g**ot **m**essage（映画『ユー・ガット・メール』から）。
Windows 専用の [mattn/gopher](https://github.com/mattn/gopher) を、
どの OS でも動くように作り直したものです。

- **何からでも話しかけられる。** シェルスクリプト、CI、cron、コーディングエージェント —
  HTTP リクエストを送れるか `gumpetctl` を実行できれば、ペットに知らせられます。
- **聞き返すこともできる。** ボタン付きの質問をペットに出して、押されたボタンを受け取れます。
  Claude Code や Codex の実行許可もこれで答えられるので、ターミナルを離れていられます。
- **ことの重さが分かる。** メッセージには info・success・warn・error があり、吹き出しの色が
  変わります。ペットも良い知らせには跳ね、悪い知らせには震えます。
- **何もないときも何か言う。** メッセージの合間に Hacker News の見出しを読み上げます。
  好きなフィードにも、何も言わないようにもできます。
- **邪魔をしない。** ウィンドウはペットと吹き出しの大きさだけ。メッセージの合間は薄くしたり
  消したりでき、吹き出しはクリックで閉じられます。
- **macOS・Windows・Linux。** バイナリ二つだけで、ほかに入れるものはありません。

## インストール

[リリースページ][releases]から使っている OS のアーカイブをダウンロードし、`gumpet` と
`gumpetctl` を `PATH` の通った場所に置きます。macOS 版は Apple Silicon と Intel の
両方で動くユニバーサルバイナリです。

### 最初の起動では警告が出ます

コード署名をしていないので、ダウンロードしたものを初めて起動すると、OS が「確認できない」と
警告します。想定どおりの動きで、次の手順で一度通せば以後は出ません。

**macOS** では、マルウェアが含まれていないことを Apple が検証できないため開けない、と
表示されます。起動する前に隔離属性を外してください:

```
xattr -d com.apple.quarantine gumpet gumpetctl
```

または一度開こうとしたあと、**システム設定 → プライバシーとセキュリティ** で
**このまま開く** を押します。

**Windows** では、SmartScreen の「Windows によって PC が保護されました」が出ます。
**詳細情報** → **実行** を選ぶか、先に PowerShell で二つのファイルのブロックを解除します:

```
Unblock-File .\gumpet.exe, .\gumpetctl.exe
```

**Linux** では何も出ません。

設定ページから入れたアップデートでは、警告は再び出ません。gumpet が自分でダウンロードするので、
ブラウザが付ける「インターネットから来た」という印が付かないためです。その代わり、gumpet の
リリース用ワークフローが署名したものかを確かめ、署名のないものは入れません。自分で確かめる
方法は [Checking a download by hand](docs/guide.md#checking-a-download-by-hand)（英語）にあります。

### ほかの入れ方

Go 1.25 以降があれば:

```
go install github.com/kaakaa/gumpet/cmd/gumpet@latest
go install github.com/kaakaa/gumpet/cmd/gumpetctl@latest
```

チェックアウトからのビルドは
[DEVELOPMENT.md](DEVELOPMENT.md)（英語）にあります。

[releases]: https://github.com/kaakaa/gumpet/releases

## はじめかた

```
gumpet
```

画面の右下にゴーファーが現れて歩きはじめます。別のターミナルから:

```
gumpetctl おなかすいた
gumpetctl -title CI -level success "All checks have passed"
go test ./... 2>&1 | tail -1 | gumpetctl -
```

クライアントを使わずに送ることもできます:

```
curl -X POST http://127.0.0.1:8787/api/v1/messages --data-binary 'hello gopher'
```

ペットをクリックするとメニューが開きます。見た目や動きは `gumpetctl -settings` で
ブラウザの設定ページから変えられ、すぐに反映されます。

## ペットを選ぶ

<p align="center">
  <img src="docs/images/pets.gif" width="660" alt="同梱の5体のペット: gopher、pixel、astro、rose、flier">
</p>

5体を同梱しています。ペットのメニュー、設定ページ、または設定ファイルの `pet.source` で
選べます。自分の絵も使えます: アニメーション GIF、PNG、フレームを入れたフォルダ。
しゃべっている間だけのフレームも用意できます。
[Using your own pet](docs/guide.md#using-your-own-pet)（英語）を見てください。

## エージェントに聞いてもらう

[contrib/claude-code](contrib/claude-code) には、[Claude Code](https://claude.com/claude-code)
の質問や「終わりました」の知らせをペットに送るフックがあります。`PermissionRequest` フックとして
`gumpetctl hook permission` を使うと、Claude がコマンドを実行してよいかの確認が **Allow** と
**Deny** のボタン付きでペットに出て、押したボタンがそのまま Claude への答えになります。
[Codex](contrib/codex) も同じ形式のフックで使えます。誰も答えなければ、エージェントは
いつもどおりターミナルで聞きます。

自分のスクリプトからも同じように聞けます:

```
gumpetctl ask -choices "Deploy,Wait" "main is green. Deploy?"
```

## ドキュメント（英語）

- **[User guide](docs/guide.md)** — すべての設定、HTTP API、ペットの動きとしゃべるタイミング、
  メニュー、自分の絵、フォント、アップデート
- **[DEVELOPMENT.md](DEVELOPMENT.md)** — ビルド、テスト、Issue からプルリクエストを作る
  ワークフロー、リリース

## ライセンス

MIT — [LICENSE](LICENSE) を見てください。gumpet が使っているライブラリ、フォント、絵にはそれぞれの
ライセンスがあり、各リリースのアーカイブに入っている `THIRD_PARTY_NOTICES` にまとめています。

### 絵について

同梱のペットにはそれぞれ `NOTICE` があり、出所と利用条件を書いています。そこでは二つを必ず
分けています: 絵そのもののライセンスと、ゴーファーというキャラクターのライセンスです。
後者は、誰が描いた絵であっても Renée French のものです。

| ペット | 絵の出所 | |
| --- | --- | --- |
| `gopher` | [mattn/gopher](https://github.com/mattn/gopher)、MIT | [NOTICE](assets/gopher/NOTICE) |
| `pixel` | [egonelbre/gophers](https://github.com/egonelbre/gophers)、CC0 1.0 | [NOTICE](assets/pixel/NOTICE) |
| `astro`、`rose`、`flier` | [Kenney](https://kenney.nl/assets/pixel-platformer)、CC0 1.0 | [NOTICE](assets/astro/NOTICE) |

二つのゴーファーは Renée French が生み出したキャラクターを描いたもので、
[CC BY 3.0](https://creativecommons.org/licenses/by/3.0/deed.ja) のもとで使っています。
CC0 は絵についての権利を放棄するもので、その絵が描いているキャラクターの権利までは放棄しません。
Kenney のスプライトは、誰のキャラクターでもありません。

同梱の画像は、ピクセルが正方形のままになるよう最近傍補間で整数倍に拡大し、複数のタイルから
なるものはアニメーションに組み立てています。それ以外は手を加えていません。この README の
画像も、それらから描いたものです。
