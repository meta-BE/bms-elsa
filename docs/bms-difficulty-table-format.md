# BMS難易度表フォーマット

## 概要

BMS難易度表は共通のフォーマット（BMSTable形式）で公開されている。
beatorajaはこの形式を読み込み、譜面のMD5ハッシュでsongdata.dbと照合して難易度を表示する。

## データ取得フロー

```
HTML (<meta name="bmstable" content="header.json">)
  → header.json (テーブルメタ情報 + data_url)
    → body JSON (譜面データ配列)
```

## header.json

```json
{
  "name": "Stella",
  "symbol": "st",
  "data_url": "score.json",
  "level_order": ["0", "1", "2", ...],
  "course": [...]
}
```

| フィールド | 必須 | 説明 | 例 |
|---|---|---|---|
| `name` | 必須 | 難易度表名 | `"Stella"`, `"発狂BMS難易度表"` |
| `symbol` | 必須 | レベル表記の接頭辞 | `"st"`, `"★"`, `"✡"` |
| `data_url` | 必須 | 本体JSONのURL（相対または絶対） | `"score.json"`, GAS URL |
| `level_order` | 任意 | レベルの表示順 | `["1","2",...,"25","???"]` |
| `course` | 任意 | 段位認定データ | MD5配列 + 制約条件 |

## body JSON（譜面データ配列）

```json
[
  {
    "md5": "9188a4c9876386173ba35158edf23a15",
    "level": "0",
    "title": "#B2FFFF [SP Celeste Colored Strawberry]",
    "artist": "tkqn14 mov. WIC / obj: Dignitas"
  },
  ...
]
```

### 共通フィールド

| フィールド | 必須 | 説明 |
|---|---|---|
| `md5` | 必須 | 譜面のMD5ハッシュ（songdata.dbとの照合キー） |
| `level` | 必須 | 難易度レベル（symbolと組み合わせて `st0`, `★12` 等） |
| `title` | ほぼ必須 | 曲名 |
| `artist` | 任意 | アーティスト名 |

### テーブル別の追加フィールド

| テーブル | 追加フィールド |
|---|---|
| Stella | `id`, `sha256`, `url`, `url_diff` |
| Solomon | `song_artist`, `charter`, `comment` |
| 発狂BMS難易度表 | `lr2_bmsid`, `url`, `url_diff`, `name_diff`, `comment` |
| like_st | `url`, `url_diff`, `ポテ`, `ヨシ`, `魔女`, `他`, `comment` |

## 調査した難易度表

| 難易度表 | symbol | data_url方式 | レベル範囲 | 譜面数 |
|---|---|---|---|---|
| [Stella](https://stellabms.xyz/st/table.html) | `st` | 静的JSON (`score.json`) | st0〜st12 | 500+ |
| [Solomon](https://mplwtch.github.io/Solomon/) | `✡` | Google Apps Script | ✡15〜✡25 | 622 |
| [like_st](https://potechang.github.io/like_st/) | `集` | Google Apps Script | 0〜2 | 550 |
| [発狂BMS難易度表](https://darksabun.club/table/archive/insane1/) | `★` | 静的JSON (`data.json`) | ★1〜★25, ??? | 1000+ |

## data_urlのホスティング方式

- **静的JSON**: 同一オリジンの相対パス。単純なfetchで取得可能。
- **Google Apps Script**: GASのexec URLを指定。302リダイレクトを経由してJSONを返す。
