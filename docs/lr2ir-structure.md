# LR2IR レスポンス構造解析

LR2IR（http://www.dream-pro.info/~lavalse/LR2IR/）の譜面ページをHTMLスクレイピングする際の知見をまとめる。

## アクセス方法

### エンドポイント

```
http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5=<MD5>
```

- HTTPのみ（HTTPSは接続拒否される）
- 文字コード: Shift_JIS
- MD5は32桁の16進数小文字（songdata.dbのsong.md5と同一）

### bmsid指定でもアクセス可能

```
http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsid=<ID>
```

bmsidはLR2IR内部のIDで、レスポンスHTML内のリンクから取得できる。

## レスポンスパターン

### 未登録の場合

ページサイズが小さく（約2KB）、body内に以下のテキストが含まれる:

```html
この曲は登録されていません。<br>
```

`<h1>`, `<h3>情報`, 情報テーブルなどは一切出力されない。

### 登録済みの場合

ページ構成:

```
<h4>ジャンル</h4>
<h1>タイトル</h1>
<h2>アーティスト</h2>

<h3>情報</h3>
  最終更新者 / 更新履歴リンク
  情報テーブル

<h3>総合ステータス</h3>    ← パース対象外
<h3>動画</h3>              ← 存在しない場合あり、パース対象外
<h3>ランキング</h3>        ← パース対象外
```

## 情報セクションの詳細構造

### ヘッダ部（h4 / h1 / h2）

```html
<h4>ELECTRO</h4>           <!-- ジャンル -->
<h1>Lovers,</h1>           <!-- タイトル -->
<h2>SHANG0</h2>            <!-- アーティスト（#ARTIST + #SUBARTIST の合成値） -->
```

- `<h2>` のアーティストは `#ARTIST` と `#SUBARTIST` の連結。例: `hurirai BGA:hrchem`

### 更新者情報

```html
<h3>情報</h3>
最終更新者 [natsuki] (2021-09-11 21:59:25) &nbsp;<a href="search.cgi?mode=editlogList&bmsid=219970">更新履歴</a>
```

- `[ユーザー名]` と `(日時)` で構成
- `[DATABASE]` は自動登録を意味する

### 情報テーブル

`<h3>情報</h3>` 直後の `<table>` に以下の行が格納される:

#### 1行目: 基本情報

```html
<tr>
  <th width="10%">BPM</th>      <td width="15%">95 - 190 </td>
  <th width="10%">レベル</th>    <td width="15%">☆21</td>
  <th width="10%">鍵盤数</th>    <td width="15%">7KEYS</td>
  <th width="10%">判定ランク</th> <td width="10%">EASY</td>
</tr>
```

| 項目 | 説明 | 値の例 |
|------|------|--------|
| BPM | 最小 - 最大 | `135 - 135`, `95 - 190` |
| レベル | 難易度表記。☆=通常, ★=発狂 | `☆10`, `☆21` |
| 鍵盤数 | プレイモード | `7KEYS`, `5KEYS`, `14KEYS` 等 |
| 判定ランク | 判定窓の広さ | `VERY HARD`, `HARD`, `NORMAL`, `EASY` |

#### 2行目: タグ

```html
<tr>
  <th>タグ</th>
  <td colspan="7">
    <a href="search.cgi?mode=search&type=tag&keyword=Stella">Stella</a>
    <a href="search.cgi?mode=search&type=tag&keyword=st2">st2</a>
    <a href="...&keyword="></a>  <!-- 空タグ（最大10スロット） -->
  </td>
</tr>
```

- 最大10個のタグスロット（`<a>` タグ）
- 空の `keyword=` はタグ未設定を意味する
- 発狂難易度表（Stella, Satellite等）への所属を示すタグが入ることが多い

#### 3行目: 本体URL

```html
<tr>
  <th>本体URL</th>
  <td colspan="7">
    <a href="https://drive.google.com/..." target="_blank">https://drive.google.com/...</a>
  </td>
</tr>
```

- 楽曲本体（音声+BGA+譜面パッケージ）のダウンロードURL
- 空の場合は `<td colspan="7"></td>`（リンクなし）
- URLの種類: Google Drive, manbow.nothing.sh（BOFイベント）, Dropbox 等

#### 4行目: 差分URL

```html
<tr>
  <th>差分URL</th>
  <td colspan="7">
    <a href="http://absolute.pv.land.to/uploader/src/up5912.zip" target="_blank">http://...</a>
  </td>
</tr>
```

- 差分譜面のダウンロードURL
- 本体URLとは別に、特定の譜面だけを配布するためのURL
- 空の場合は `<td colspan="7"></td>`

#### 5行目（任意）: 備考

```html
<tr>
  <th>備考</th>
  <td colspan="7">汎用up5912</td>
</tr>
```

- 自由テキスト。差分の補足情報が書かれることが多い
- **本体URL・差分URLが空でも備考だけ存在する場合がある**
- 備考行自体が存在しないページもある

## 検証結果サンプル

### パターン1: 最小限の情報（URL空）

**md5**: `d91af3b677cd97d8dbed7ab3e3bae244` / **bmsid**: 112470

| 項目 | 値 |
|------|-----|
| ジャンル | ELECTRO |
| タイトル | Lovers, |
| アーティスト | SHANG0 |
| BPM | 135 - 135 |
| レベル | ☆10 |
| 鍵盤数 | 7KEYS |
| 判定ランク | NORMAL |
| タグ | （空） |
| 本体URL | （空） |
| 差分URL | （空） |
| 備考 | （行なし） |
| 更新者 | [DATABASE] (2016-09-11) |

### パターン2: 本体URLあり

**md5**: `0fe696ac60e831fc111285d6099eadbb` / **bmsid**: 235711

| 項目 | 値 |
|------|-----|
| ジャンル | Starburst |
| タイトル | Andromeda [SP Another] |
| アーティスト | hurirai BGA:hrchem |
| BPM | 193 - 193 |
| レベル | ☆12 |
| 鍵盤数 | 7KEYS |
| 判定ランク | EASY |
| タグ | （空） |
| 本体URL | https://drive.google.com/file/d/0B2b58uoHEDp7bGZqdUFkeUxOUU0/view?usp=sharing |
| 差分URL | （空） |
| 備考 | （行なし） |
| 更新者 | [saaa] (2018-11-11) |

### パターン3: 本体URL + 差分URL + 備考あり

**md5**: `c7fb88a21280b2d0de8f477036f43225` / **bmsid**: 219970

| 項目 | 値 |
|------|-----|
| ジャンル | DEMENTIA PROVECTO |
| タイトル | Anima Mundi -Incipiens Finis- [EX] |
| アーティスト | Limstella BGA:アカツキユウ / obj:仙人掌の人 |
| BPM | 95 - 190 |
| レベル | ☆21 |
| 鍵盤数 | 7KEYS |
| 判定ランク | EASY |
| タグ | Stella, st2 |
| 本体URL | http://manbow.nothing.sh/event/event.cgi?action=More_def&num=78&event=104 |
| 差分URL | http://absolute.pv.land.to/uploader/src/up5912.zip |
| 備考 | 汎用up5912 |
| 更新者 | [natsuki] (2021-09-11) |

### パターン4: 未登録

**md5**: `dc69ce88fadfe76e0a33a1646c3e0923`（Innocent warrior）

LR2IRに未登録。レスポンスボディに `この曲は登録されていません。` のみ。

## パース時の注意点

1. **文字コード**: レスポンスはShift_JIS。UTF-8への変換が必要
2. **HTMLエンティティ**: URL内の `&` が `&amp;` にエスケープされている場合がある
3. **備考行の有無**: 備考行は任意。情報テーブルの行数が可変
4. **タグの空スロット**: `keyword=` が空の `<a>` タグはフィルタリングが必要
5. **未登録判定**: `この曲は登録されていません。` の文字列で判定可能
6. **アーティスト欄**: `#ARTIST` + `#SUBARTIST` の合成値。パース時にsongdata.dbの値と差異がありうる
7. **レートリミット**: 不明。大量アクセスは避けるべき（元々LR2プレイヤー向けの個人運営サイト）
