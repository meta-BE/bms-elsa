# BMS SEARCH API 仕様

BMS SEARCH（https://bmssearch.net/）のREST APIの仕様をまとめる。BMS・譜面・アーティスト・イベントの検索が可能なデータベースサービス。

APIドキュメント: https://doc.api.bmssearch.net/

## API基本情報

| 項目 | 値 |
|------|-----|
| ベースURL | `https://api.bmssearch.net/v1` |
| バージョン | 1.0.0 |
| 認証 | なし（APIキー不要） |
| レートリミット | 明示なし（常識的な範囲で利用） |
| CORS | `access-control-allow-origin: *` |
| レスポンス形式 | JSON（UTF-8） |
| 仕様変更告知 | https://bmssearch.notion.site/2b76b1578d3f42c1af6311b24b761878 |

## bms-elsaに関連するエンドポイント

### MD5で譜面を取得

```
GET /patterns/{hashMd5}
```

songdata.dbの `song.md5` カラムをそのまま使える。bms-elsaとの連携で最も重要なエンドポイント。

**リクエスト例:**
```
GET https://api.bmssearch.net/v1/patterns/59e8cd9f52a55d8cdd439f40aa74b555
```

**レスポンス（200 OK）:**
```json
{
  "bms": {
    "id": "Y--E3erQ36Ap-F",
    "title": "SUNDAY"
  },
  "format": "CONVENTIONAL",
  "genre": "FUNK POP",
  "title": "SUNDAY",
  "subtitles": [],
  "artist": "MIKE",
  "subartists": [],
  "playlevel": 6,
  "laneType": "B_5K",
  "totalNotes": 457,
  "bpm": {
    "min": 125,
    "max": 125
  },
  "file": {
    "name": "sunday.bms",
    "size": 34834,
    "extension": ".bms",
    "hashMd5": "59e8cd9f52a55d8cdd439f40aa74b555",
    "hashSha256": "59bab147831afa60dee3a8caae4bac3df4c7b97679adbd9fdf981503bbdf5ed1"
  },
  "description": "",
  "packType": "INCLUDED",
  "tags": [],
  "createdAt": "2021-06-12T09:51:21.244Z",
  "updatedAt": "2022-03-14T02:14:44.839Z"
}
```

**未登録の場合（404 Not Found）:**
```json
{ "message": "Not Found" }
```

### SHA256で譜面を取得

```
GET /patterns/sha256/{hashSha256}
```

MD5検索と同一のPatternオブジェクトを返す。beatorajaのsongdata.dbには `song.sha256` カラムがあるため利用可能。

### BMS詳細を取得

```
GET /bmses/{id}
```

Patternレスポンスの `bms.id` を使って親作品の詳細情報を取得する。ダウンロードURL、プレビュー動画、イベント情報などPatternにはない情報が含まれる。

**レスポンス例:**
```json
{
  "id": "rGHe-aYOskGqqo",
  "exhibition": {
    "id": "55rvqQDu81DFOn",
    "name": "BMSをたくさん作るぜ'22"
  },
  "genre": "Nostalgic J-POP",
  "title": "杪冬の願い",
  "artist": "あとぅす feat.橘花音",
  "subartist": "",
  "publishedAt": "2022-03-09T05:15:00.000Z",
  "downloads": [
    {
      "url": "https://www.dropbox.com/s/4lz2ju9whnr1xqc/%5Batos%5Dbyoutou_no_negai_ogg2.zip?dl=1",
      "description": ""
    }
  ],
  "previews": [
    { "service": "YOUTUBE", "parameter": "CK0er9iNNos" }
  ],
  "relatedLinks": [
    { "url": "https://venue.bmssearch.net/bmstukuru2022/33", "description": "会場" }
  ],
  "bemuseURL": "",
  "tags": [],
  "createdAt": "2022-03-15T05:16:19.323Z",
  "updatedAt": "2022-04-19T08:54:34.439Z"
}
```

### BMS配下の全譜面を取得

```
GET /bmses/{id}/patterns
```

1つのBMS作品に含まれる全譜面（5KEY、7KEY、10KEY、差分など）を配列で返す。

**レスポンス例（4譜面を含むBMS）:**
```json
[
  { "title": "SUNDAY [7Keys]", "laneType": "B_7K", "file": { "hashMd5": "40deccc7f1fe126d19eeee1629e21f9a", ... } },
  { "title": "SUNDAY (7keys Easy)", "laneType": "B_5K", "file": { "hashMd5": "4c9f489d2718592cb443b1c624062413", ... } },
  { "title": "SUNDAY", "laneType": "B_5K", "file": { "hashMd5": "59e8cd9f52a55d8cdd439f40aa74b555", ... } },
  { "title": "SUNDAY [BMSSP 10keys]", "laneType": "B_10K", "file": { "hashMd5": "b638157674d57e2b464273874cb22707", ... } }
]
```

## レスポンスJSONスキーマ

### Pattern オブジェクト

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `bms` | object | 親BMS作品 `{ id: string, title: string }` |
| `format` | string | `"CONVENTIONAL"` \| `"BMSON"` |
| `genre` | string | ジャンル |
| `title` | string | 譜面タイトル（サブタイトル込み） |
| `subtitles` | string[] | サブタイトル配列 |
| `artist` | string | アーティスト |
| `subartists` | string[] | サブアーティスト配列（例: `["BGI:りーふぱいザウルス"]`） |
| `difficulty` | string? | 難易度名（例: `"ANOTHER"`）。存在しない場合あり |
| `playlevel` | integer | 譜面レベル |
| `laneType` | string | `"B_5K"` \| `"B_7K"` \| `"B_10K"` \| `"B_14K"` \| `"P_5K"` \| `"P_9K"` \| `"P_18K"` \| `"UNKNOWN"` |
| `totalNotes` | integer | 総ノーツ数 |
| `bpm` | object | `{ min: number, max: number }` |
| `file` | object | ファイル情報（下記参照） |
| `description` | string | 譜面説明 |
| `packType` | string | `"INCLUDED"`（本体同梱）\| `"ADDITIONAL"`（差分） |
| `tags` | string[] | タグ配列 |
| `createdAt` | string | 作成日時（ISO 8601） |
| `updatedAt` | string | 更新日時（ISO 8601） |

### File オブジェクト（Pattern内）

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `name` | string | ファイル名（例: `"sunday.bms"`） |
| `size` | integer | ファイルサイズ（バイト） |
| `extension` | string | 拡張子（例: `".bms"`, `".bme"`, `".bml"`） |
| `hashMd5` | string | MD5ハッシュ（32桁16進数小文字） |
| `hashSha256` | string | SHA256ハッシュ（64桁16進数小文字） |

### BMS オブジェクト

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `id` | string | BMS ID |
| `exhibition` | object? | 所属イベント `{ id: string, name: string }`。null の場合あり |
| `genre` | string | ジャンル |
| `title` | string | 作品タイトル |
| `artist` | string | アーティスト |
| `subartist` | string | サブアーティスト |
| `publishedAt` | string | 公開日時（ISO 8601） |
| `downloads` | object[] | ダウンロードURL配列 `[{ url: string, description: string }]` |
| `previews` | object[] | プレビュー配列 `[{ service: string, parameter: string }]` |
| `relatedLinks` | object[] | 関連リンク配列 `[{ url: string, description: string }]` |
| `bemuseURL` | string | Bemuse プレイURL |
| `tags` | string[] | タグ配列 |
| `createdAt` | string | 作成日時（ISO 8601） |
| `updatedAt` | string | 更新日時（ISO 8601） |

### Preview オブジェクト

| service | parameter の意味 |
|---------|-----------------|
| `"YOUTUBE"` | YouTube動画ID（例: `"TcwOpsnXbNE"` → `https://youtube.com/watch?v=TcwOpsnXbNE`） |
| `"SOUNDCLOUD"` | SoundCloud トラックURL |
| `"NICONICO"` | ニコニコ動画ID |

## エラーレスポンス

### 404 Not Found
```json
{ "message": "Not Found" }
```

### 400 Bad Request（バリデーションエラー）
```json
{
  "error": [
    {
      "path": "/query/limit",
      "message": "must be <= 100",
      "errorCode": "maximum.openapi.validation"
    }
  ]
}
```

### 412 Precondition Failed
```json
{ "message": "Precondition Failed" }
```
MinHashデータが未登録の譜面に対して `/patterns/sha256/{hash}/neighbors` を呼んだ場合。

## ページネーション

検索系エンドポイント（`/bmses/search`, `/patterns/search` 等）で使用。

| パラメータ | 型 | 範囲 | 説明 |
|-----------|-----|------|------|
| `offset` | integer | 0〜 | 開始位置 |
| `limit` | integer | 0〜100 | 取得件数（最大100） |

- レスポンスは純粋なJSON配列。total countなどのメタデータは含まれない
- 次ページ: `offset += limit` で再リクエスト
- 結果が `limit` 未満なら最終ページ

## 全エンドポイント一覧

### BMS

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/bmses/features` | FEATURED BMS一覧 |
| GET | `/bmses/search` | BMS検索 |
| GET | `/bmses/{id}` | BMS詳細 |
| GET | `/bmses/{id}/description` | 説明文 |
| GET | `/bmses/{id}/patterns` | 配下の全譜面 |
| GET | `/bmses/{id}/likedCount` | いいね数 |
| GET | `/bmses/{id}/relatives` | 類似BMS（現在非機能） |
| GET | `/bmses/legacy/{legacyId}` | 旧ID→新ID |

### 譜面（Pattern）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/patterns/{hashMd5}` | **MD5で譜面取得** |
| GET | `/patterns/sha256/{hashSha256}` | **SHA256で譜面取得** |
| GET | `/patterns/search` | 譜面検索 |
| GET | `/patterns/sha256/{hashSha256}/neighbors` | MinHash類似譜面（実験的） |
| GET | `/patterns/legacy/{legacyId}` | 旧ID |
| PUT | `/patterns` | 譜面アップロード・解析 |

### アーティスト

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/artists/{id}` | 詳細 |
| GET | `/artists/{id}/description` | 説明文 |
| GET | `/artists/{id}/identicals` | 別名義一覧 |
| GET | `/artists/{id}/relatives` | 関連名義 |
| GET | `/artists/{id}/labels` | 所属レーベル |
| GET | `/artists/random` | ランダム |
| GET | `/artists/search` | 検索 |
| GET | `/artists/resolve` | 表記分解 |
| GET | `/artists/legacy/{legacyId}` | 旧ID |

### イベント（Exhibition）

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/exhibitions/{id}` | 詳細 |
| GET | `/exhibitions/{id}/bmses` | 参加BMS一覧 |
| GET | `/exhibitions/{id}/description` | 説明文 |
| GET | `/exhibitions/search` | 検索 |
| GET | `/exhibitions/legacy/{legacyId}` | 旧ID |

### レーベル

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/labels/{id}` | 詳細 |
| GET | `/labels/{id}/description` | 説明文 |
| GET | `/labels/{id}/artists` | 所属アーティスト |
| GET | `/labels/search` | 検索 |
| GET | `/labels/legacy/{legacyId}` | 旧ID |

### プレイリスト

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/playlists/{id}` | 概要 |
| GET | `/playlists/{id}/bmses` | BMS一覧 |
| GET | `/playlists/search` | 検索 |

### ユーザー

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/users/{id}` | ユーザー情報 |
| GET | `/users/{id}/playlists` | 公開プレイリスト |
| GET | `/users/{id}/liked/bmses` | いいね済みBMS |

## 検索パラメータ

### `/bmses/search`

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `q` | string | キーワード（タイトル/アーティスト部分一致AND検索） |
| `genre` | string | ジャンル |
| `title` | string | タイトル |
| `artist` | string | アーティスト |
| `subartist` | string | サブアーティスト |
| `exhibition` | string | イベント名 |
| `tag` | string | タグ |
| `publishedAtFrom` | string | 公開日時（From、ISO 8601） |
| `publishedAtTo` | string | 公開日時（To、ISO 8601） |
| `filters` | string | `HAS_PREVIEWS` \| `HAS_ARTWORKS` \| `HAS_DOWNLOADS` \| `HAS_ADDITIONAL_PATTERNS` |
| `orderBy` | string | `CREATED` \| `UPDATED` \| `PUBLISHED` |
| `orderDirection` | string | `DESC` \| `ASC` |
| `offset` | integer | 開始位置（最小0） |
| `limit` | integer | 取得件数（最小0、最大100） |

### `/patterns/search`

| パラメータ | 型 | 説明 |
|-----------|-----|------|
| `q` | string | キーワード（スペース区切りでOR検索） |
| `title` | string | タイトル |
| `subtitle` | string | サブタイトル |
| `artist` | string | アーティスト |
| `subartist` | string | サブアーティスト |
| `genre` | string | ジャンル |
| `tag` | string | タグ |
| `levelMin` / `levelMax` | number | レベル範囲 |
| `totalNotesMin` / `totalNotesMax` | integer | ノーツ数範囲 |
| `bpmMin` / `bpmMax` | number | BPM範囲 |
| `format` | string | `CONVENTIONAL` \| `BMSON` |
| `packType` | string | `INCLUDED` \| `ADDITIONAL` |
| `laneType` | string | `B_5K` \| `B_7K` \| `B_10K` \| `B_14K` \| `P_5K` \| `P_9K` \| `P_18K` \| `UNKNOWN` |
| `fileExtension` | string | 拡張子（例: `.bme`） |
| `fileName` | string | ファイル名 |
| `filters` | string | `BMS_LINKED` \| `BMS_UNLINKED` |
| `offset` | integer | 開始位置 |
| `limit` | integer | 取得件数（最大100） |

## LR2IRとの比較

| 項目 | LR2IR | BMS SEARCH |
|------|-------|------------|
| プロトコル | HTTP（HTMLスクレイピング） | HTTPS（REST API + JSON） |
| 認証 | なし | なし |
| 文字コード | Shift_JIS | UTF-8 |
| 検索キー | MD5 / bmsid | MD5 / SHA256 / ID |
| 一括取得 | 不可（1件ずつ） | 不可（1件ずつ）。検索APIで条件付き一括取得は可能 |
| ダウンロードURL | あり（本体URL / 差分URL） | あり（`/bmses/{id}` の `downloads` フィールド。Pattern→BMS→downloads の2段階） |
| イベント情報 | なし | あり（`exhibition` フィールド） |
| タグ | あり（最大10個） | あり（配列） |
| ノーツ数 | なし | あり（`totalNotes`） |
| 鍵盤種別 | あり（文字列） | あり（`laneType` enum） |
| 譜面カバー率 | 高（古い譜面も多い） | 中〜高（testdata/songdata.dbで約70%ヒット） |

## 注意点

1. **一括MD5検索APIは存在しない**: 1件ずつGETリクエストを送る必要がある。大量取得時はレート制限に注意
2. **ダウンロードURLはPatternではなくBMSに紐づく**: MD5で譜面を取得し、`bms.id`でBMS詳細を取得する2段階のリクエストが必要
3. **全譜面がBMS SEARCHに登録されているわけではない**: 未登録譜面は404を返す。カバー率は約70%（testdata/songdata.dbでの検証）
4. **`difficulty`フィールドは任意**: 一部のPatternにのみ存在する（例: `"ANOTHER"`）
5. **`subtitles` / `subartists` は配列**: LR2IRの文字列連結とは異なり、構造化されている
