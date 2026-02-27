# beatoraja songdata.db スキーマ解析

beatorajaが楽曲データ管理に使用するSQLiteデータベースの構造と内容をまとめる。

## テーブル一覧

| テーブル | レコード数 | 概要 |
|----------|-----------|------|
| song | 15,437 | 楽曲（譜面単位）のメタデータ |
| folder | 2,712 | 楽曲フォルダの階層構造 |

---

## songテーブル

### スキーマ

```sql
CREATE TABLE [song] (
  [md5] TEXT NOT NULL,
  [sha256] TEXT NOT NULL,
  [title] TEXT,
  [subtitle] TEXT,
  [genre] TEXT,
  [artist] TEXT,
  [subartist] TEXT,
  [tag] TEXT,
  [path] TEXT,          -- PRIMARY KEY
  [folder] TEXT,
  [stagefile] TEXT,
  [banner] TEXT,
  [backbmp] TEXT,
  [preview] TEXT,
  [parent] TEXT,
  [level] INTEGER,
  [difficulty] INTEGER,
  [maxbpm] INTEGER,
  [minbpm] INTEGER,
  [length] INTEGER,
  [mode] INTEGER,
  [judge] INTEGER,
  [feature] INTEGER,
  [content] INTEGER,
  [date] INTEGER,
  [favorite] INTEGER,
  [adddate] INTEGER,
  [notes] INTEGER,
  [charthash] TEXT,
  PRIMARY KEY(path)
);
```

### カラム詳細

#### 識別子

| カラム | 型 | 説明 | 備考 |
|--------|------|------|------|
| path | TEXT (PK) | BMSファイルの絶対パス | `G:\BMS\SONGS\...\file.bme` 形式 |
| md5 | TEXT | BMSファイルのMD5ハッシュ | LR2IRでの譜面識別に使用。NULL27件 |
| sha256 | TEXT | BMSファイルのSHA-256ハッシュ | beatorajaでの譜面識別に使用 |
| charthash | TEXT | 譜面データ（ノーツ配置）のSHA-256 | 全件存在 |

#### 楽曲メタデータ

| カラム | 型 | 説明 | 空率 | 備考 |
|--------|------|------|------|------|
| title | TEXT | 楽曲タイトル | 4件空 | BMSヘッダの`#TITLE` |
| subtitle | TEXT | サブタイトル | - | `[ANOTHER]`や`[FREQ -1.0]`など |
| genre | TEXT | ジャンル | 116件空 | BMSヘッダの`#GENRE` |
| artist | TEXT | アーティスト | 31件空 | BMSヘッダの`#ARTIST` |
| subartist | TEXT | サブアーティスト | 9,004件空(58%) | `obj:譜面作者 :: BGA:BGA作者` 形式 |
| tag | TEXT | タグ | 全件空 | 未使用 |

#### メディアファイル参照

| カラム | 使用件数 | 説明 |
|--------|---------|------|
| stagefile | 10,503 (68%) | ステージ画像（読み込み画面等） |
| banner | 6,078 (39%) | バナー画像 |
| backbmp | 3,307 (21%) | 背景画像 |
| preview | 3,106 (20%) | プレビュー音声 |

#### 譜面属性

| カラム | 型 | 説明 |
|--------|------|------|
| level | INTEGER | 譜面レベル（作者定義） |
| difficulty | INTEGER | 難易度区分（後述） |
| notes | INTEGER | ノーツ数 |
| maxbpm | INTEGER | 最大BPM |
| minbpm | INTEGER | 最小BPM |
| length | INTEGER | 楽曲長さ（ミリ秒）。範囲: 12,315〜799,999 |
| mode | INTEGER | プレイモード（後述） |
| judge | INTEGER | 判定倍率。100が標準、75=EASY, 50=VERY EASY |

#### 分類・フラグ

| カラム | 型 | 説明 |
|--------|------|------|
| folder | TEXT | 所属フォルダの8桁ハッシュ（folderテーブルと対応） |
| parent | TEXT | 親グループの8桁ハッシュ（イベント等の上位階層） |
| feature | INTEGER | 譜面特徴のビットフラグ（後述） |
| content | INTEGER | コンテンツ種別のビットフラグ（後述） |
| favorite | INTEGER | お気に入りフラグ。0=未設定 |
| date | INTEGER | BMSファイルの更新日時（Unixタイムスタンプ秒） |
| adddate | INTEGER | beatorajaへの登録日時（Unixタイムスタンプ秒） |

---

### difficulty（難易度区分）

| 値 | 名称 | 件数 |
|----|------|------|
| 1 | BEGINNER | 1,058 |
| 2 | NORMAL | 3,006 |
| 3 | HYPER | 3,101 |
| 4 | ANOTHER | 3,820 |
| 5 | INSANE | 4,440 |
| 6〜 | 不正値/カスタム | 12 |

### mode（プレイモード）

| 値 | 説明 | 件数 |
|----|------|------|
| 5 | 5KEYS (beat-5k) | 673 |
| 7 | 7KEYS (beat-7k) | 12,087 |
| 9 | 9KEYS (popn) | 774 |
| 10 | 10KEYS (beat-5k DP) | 116 |
| 14 | 14KEYS (beat-7k DP) | 1,785 |
| 25 | 24KEYS+SC (keyboard) | 2 |

### feature（譜面特徴ビットフラグ）

| ビット | 値 | 意味 |
|--------|------|------|
| bit 0 | 1 | UNDEFINEDLN（未定義ロングノート） |
| bit 1 | 2 | MINENOTE（地雷ノート） |
| bit 2 | 4 | RANDOM（ランダム分岐） |
| bit 3 | 8 | LONGNOTE（ロングノート） |
| bit 4 | 16 | （用途不明） |
| bit 5 | 32 | （用途不明） |
| bit 6 | 64 | STOPSEQUENCE（ストップシーケンス） |
| bit 7 | 128 | （用途不明） |

主要な組み合わせ:

| feature値 | フラグ | 件数 |
|-----------|--------|------|
| 0 | なし | 8,469 |
| 1 | UNDEFINEDLN | 5,828 |
| 8 | LONGNOTE | 328 |
| 65 | UNDEFINEDLN + STOPSEQUENCE | 278 |
| 64 | STOPSEQUENCE | 218 |

### content（コンテンツ種別ビットフラグ）

| ビット | 値 | 意味 | 件数 |
|--------|------|------|------|
| bit 0 | 1 | WAV定義あり | - |
| bit 1 | 2 | BGA定義あり | - |

| content値 | 意味 | 件数 |
|-----------|------|------|
| 3 | WAV + BGA | 10,698 |
| 2 | BGAのみ | 3,960 |
| 1 | WAVのみ | 476 |
| 0 | なし | 303 |

---

## folderテーブル

### スキーマ

```sql
CREATE TABLE [folder] (
  [title] TEXT,
  [subtitle] TEXT,
  [command] TEXT,
  [path] TEXT,          -- PRIMARY KEY
  [banner] TEXT,
  [parent] TEXT,
  [type] INTEGER,
  [date] INTEGER,
  [adddate] INTEGER,
  [max] INTEGER,
  PRIMARY KEY(path)
);
```

### カラム詳細

| カラム | 型 | 説明 | 備考 |
|--------|------|------|------|
| path | TEXT (PK) | フォルダの絶対パス | 末尾`\`付き |
| title | TEXT | フォルダ表示名 | ディレクトリ名と同じことが多い |
| subtitle | TEXT | サブタイトル | 全件空 |
| command | TEXT | コマンド | 全件空 |
| banner | TEXT | バナー画像 | 全件空 |
| parent | TEXT | 親グループの8桁ハッシュ | 全件に値あり |
| type | INTEGER | フォルダ種別 | 全件0 |
| date | INTEGER | フォルダの更新日時（Unixタイムスタンプ秒） | |
| adddate | INTEGER | 登録日時（Unixタイムスタンプ秒） | |
| max | INTEGER | 不明 | 全件0 |

---

## テーブル間のリレーション

### song ↔ folder の対応

```
song.path LIKE folder.path || '%'
```

- songのpathはfolderのpathを先頭に含む（BMSファイルがそのフォルダ内に存在する）
- `song.folder`は楽曲が直接所属するフォルダの8桁ハッシュ（folderテーブルの最深一致パスに対応）

### parent による階層グループ

```
song.parent = folder.parent（同じ親グループに属する）
```

- `parent`は8桁の16進ハッシュで、beatorajaがスキャン対象ディレクトリのパスから生成したもの（CRC32等と推定）
- 同じparentを持つfolder群 = 同一イベント配下の楽曲フォルダ群
- 同じparentを持つsong群 = 同一イベント配下の全譜面
- **parentハッシュ自体はfolderテーブルにレコードとして存在しない**（folderテーブルは楽曲直属フォルダのみを格納）
- parentが指す実ディレクトリは、同じparentを持つ子フォルダのpathの共通prefixから復元可能

parentごとの楽曲数と対応ディレクトリ（上位10件）:

| parent | フォルダ数 | 楽曲数 | 対応ディレクトリ |
|--------|-----------|--------|-----------------|
| 7c2f7fc4 | 596 | 2,505 | `G:\BMS\SONGS\genocide\` |
| 9084fcb7 | 387 | 2,534 | `G:\BMS\SONGS\stsl\` |
| 1117ac22 | 155 | 896 | `G:\BMS\SONGS\others\` |
| 77f7351a | 92 | 637 | `G:\BMS\SONGS\BOF21-2025\` |
| 84fc2503 | 90 | 573 | `G:\BMS\SONGS\BOFU2016\` |
| 86ba9b5a | 88 | 584 | `G:\BMS\SONGS\BOFU2015\` |
| 97c0337f | 86 | 513 | `G:\BMS\SONGS\BOFXVII2021\` |
| 853e4f34 | 86 | 550 | `G:\BMS\SONGS\BOFU2017\` |
| b8f49ba7 | 82 | - | `G:\BMS\SONGS\2024\` |
| b936f190 | 81 | - | `G:\BMS\SONGS\2025\` |

### 同一楽曲の複数譜面

同一フォルダ内の複数BMSファイルが同一楽曲の異なる難易度・モードの譜面となる。
これらは同じ`folder`値を共有する。

例: `Chronomia` — BEGINNER〜INSANEまで15譜面、DPやsubtitle付き差分も含む
```
folder=e1c4787e
├── [EASY7]          difficulty=1(BEGINNER) level=3   mode=7(7KEYS)  notes=417
├── [NORMAL7]        difficulty=2(NORMAL)   level=7   mode=7(7KEYS)  notes=834
├── [pass]           difficulty=2(NORMAL)   level=23  mode=7(7KEYS)  notes=3304
├── [HYPER7]         difficulty=3(HYPER)    level=10  mode=7(7KEYS)  notes=1254
├── [ANOTHER7]       difficulty=4(ANOTHER)  level=12  mode=7(7KEYS)  notes=1891
├── [DPA] (subtitle) difficulty=4(ANOTHER)  level=12  mode=14(DP)    notes=1707
├── [Pocket watch]   difficulty=4(ANOTHER)  level=12  mode=7(7KEYS)  notes=2545
├── [Alicia]         difficulty=5(INSANE)   level=15  mode=7(7KEYS)  notes=2300
├── [Ecstacy]        difficulty=5(INSANE)   level=15  mode=7(7KEYS)  notes=3381
├── [INSANE7]        difficulty=5(INSANE)   level=16  mode=7(7KEYS)  notes=2228
├── [時]             difficulty=5(INSANE)   level=18  mode=7(7KEYS)  notes=1761
├── [Presea]         difficulty=5(INSANE)   level=19  mode=7(7KEYS)  notes=2850
├── [Cuculus..] (st) difficulty=5(INSANE)   level=20  mode=7(7KEYS)  notes=2883
├── -LAST BOSS-      difficulty=5(INSANE)   level=25  mode=7(7KEYS)  notes=3600
└── (無印)           difficulty=7(不明)     level=7   mode=7(7KEYS)  notes=1880
```

注目点:
- 同一difficulty内に複数譜面（ANOTHER×3、INSANE×7）が存在しうる
- subtitle列に差分名が入る場合がある（`[DPA]`, `[Cuculus poliocephalus]`）
- SP(7KEYS)とDP(14KEYS)が同一フォルダに混在する
- difficulty=7のような規格外の値も存在する

---

## サンプルデータ

### song（結合クエリ）

```sql
SELECT s.title, s.artist, s.genre, s.level, s.difficulty, s.mode, s.notes,
       s.maxbpm, s.length, s.md5, f.title as folder_title, f.path as folder_path
FROM song s
JOIN folder f ON s.path LIKE f.path || '%'
WHERE f.path = (
    SELECT f2.path FROM folder f2
    WHERE s.path LIKE f2.path || '%'
    ORDER BY length(f2.path) DESC LIMIT 1
)
LIMIT 5;
```

| title | artist | genre | level | difficulty | mode | notes | maxbpm | folder_title |
|-------|--------|-------|-------|------------|------|-------|--------|--------------|
| Love & Justice [EXTREME] | フロン / ラズベリル / obj: black train | LOVELY HIGHSPEED | 16 | 5 (INSANE) | 7 (7KEYS) | 2,646 | 188 | Love & Justice [EXTREME] [FREQ -1.0] |
| STAR TRACK (NiraicA_nai Mix) | roop from STR | DISCO HOUSE | 5 | 2 (NORMAL) | 5 (5KEYS) | 430 | 130 | STAR TRACK (NiraicA_nai Mix) - roop |
| SUNDAY | MIKE | FUNK POP | 6 | 2 (NORMAL) | 5 (5KEYS) | 457 | 125 | mike_sunday |

### folder

| path | title | parent |
|------|-------|--------|
| `G:\BMS\SONGS\practice\LJ EXT\Love & Justice [EXTREME] [FREQ -1.0]\` | Love & Justice [EXTREME] [FREQ -1.0] | f5e9248c |
| `G:\BMS\SONGS\BMSSP2009\STAR TRACK (NiraicA_nai Mix) - roop\` | STAR TRACK (NiraicA_nai Mix) - roop | 7eba0dae |
| `G:\BMS\SONGS\G2R2018\[G2R2018]daisan_deepsea_moonlight[RetunedColors]\` | [G2R2018]daisan_deepsea_moonlight[RetunedColors] | 86ce7a7c |

---

## bms-elsaでの活用に向けた所見

- **主キーはpath**だが、BMSコミュニティではmd5/sha256が譜面識別に広く使われる
- **song.folder**で同一フォルダ内の譜面をグルーピングできる（＝同一楽曲パッケージ）
- **song.parent / folder.parent**でイベント単位のグルーピングが可能
- **pathはWindows形式**（`G:\BMS\SONGS\...`）。実際の楽曲ファイル操作時にはパス変換が必要
- **songdata.dbは読み取り専用**で利用し、bms-elsa側で独自のDBを持つ設計が妥当
- lengthはミリ秒単位。大半の楽曲は2〜3分（120,000〜180,000ms）に分布
