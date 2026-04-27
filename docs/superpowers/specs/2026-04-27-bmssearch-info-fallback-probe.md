# BMS Search フォールバック検索 調査レポート（2026-04-27実施）

## サンプル選定基準

- 昇順5件: song テーブルを md5 昇順で先頭5件取得（md5・title ともに非空のレコードのみ）
- 降順5件: song テーブルを md5 降順で先頭5件取得（同上）
- ヒット/ミスの区別は songdata.db から参照不可（song_meta が elsa DB にあるため）
- 出力 JSON の Count・Items を目視で確認し、BMS Search での紐付け可否を判定

## サンプル一覧と結果

### 昇順5件

| md5 (先頭8文字) | Title | Artist | raw | normalized | stripped |
|---|---|---|---|---|---|
| 0003e38c | 外への鍵と躁鬱 | uet | count=1 ✓完全一致 | count=1 ✓完全一致 | count=1 ✓完全一致 |
| 0008a727 | After School Dessert(Uzawa&Kazusa HardCore Remix)[SP Semla] | KARUT... | count=0 | count=0 | count=0 ※stripped="After School Dessert" |
| 000a5c33 | Halcyon[LIGHT] | xi | count=0 | count=0 | count=2 (Halcyon関連2件、本体タイトル違い) |
| 000b825d | unstable delusional beliefs | Lamanya | count=1 ✓完全一致 | count=1 ✓完全一致 | count=1 ✓完全一致 |
| 000dda0e | ENTANGLEMENT(EX) | ABE3/さ | count=0 | count=0 | count=1 (ENTANGLEMENT、artist部分一致) |

### 降順5件

| md5 (先頭8文字) | Title | Artist | raw | normalized | stripped |
|---|---|---|---|---|---|
| ffe87cf8 | セクシー・セクシー・ダイナマイト | kei_iwata... | count=1 ✓完全一致 | count=1 ✓完全一致 | count=1 ✓完全一致 |
| fff439ea | Aqua Regia Squall [F] | DJ owl-light vs xi | count=0 | count=0 | count=1 (Aqua Regia Squall、artist完全一致) |
| fffcdd50 | Apocaliptix [Doomsayer] | Juggernaut. | count=0 | count=0 | count=1 (Apocaliptix、artist完全一致) |
| fffd5534 | Suffering of screw | Sakamiya feat.遊左 未 | count=1 ✓完全一致 | count=1 ✓完全一致 | count=1 ✓完全一致 |
| fffd7afa | energy night [Uchither] | KAH... | count=0 | count=0 | count=1 (energy night、artistが異なる) |

## 末尾付帯文字列パターン（観測値）

| パターン | 観測例 |
|---|---|
| `[難易度記号]` | `Halcyon[LIGHT]`, `Aqua Regia Squall [F]`, `Apocaliptix [Doomsayer]`, `energy night [Uchither]` |
| `(リミックス情報)` | `After School Dessert(Uzawa&Kazusa HardCore Remix)[SP Semla]` |
| `(バリアント)` | `ENTANGLEMENT(EX)` |

**観察**: `[...]` が最も多く出現。これを剥離すれば BMS Search の本体タイトルに到達できる場合がある（4件中3件で stripped によるヒットが増加）。ただし `After School Dessert` のような長いリミックス情報が `(...)` 内に入っている場合はそもそも原題が存在しない（別曲扱い）。

## 各バリアントの評価

- **raw クエリ（原文）**: 完全一致であれば高精度にヒット。BMS Search のタイトルが songdata.db と一致している場合は有効（4件がこのパターン）。末尾の難易度区別記号が付くとヒット0。
- **normalized クエリ（小文字化）**: BMS Search 側が全角日本語を保持しているため ASCII 小文字化は日本語タイトルに効果なし。ASCII タイトルでも "外への鍵と躁鬱" や "unstable delusional beliefs" 等は raw と同じ結果。今回のサンプルでは raw と完全に同じ結果。
- **stripped クエリ（末尾剥離）**: raw でヒット0だった5件のうち3件（Halcyon, ENTANGLEMENT, Aqua Regia Squall, Apocaliptix, energy night）で候補が出現。ただし stripped ヒット時のスコアリングが重要。

## 正規化ルール最終案

観察結果から以下のルールを採用する。

1. **ケース折りたたみ**: ASCII 大小無視（今回のサンプルでは `normalized` が raw と同じ結果で、BMS Search 側が原文を保持しているため効果が限定的）
2. **全半角統一**: NFKC 適用（energy night のartistで全角スペースが存在した。タイトル検索で全半角が問題になるケースは今回未確認だが、将来のために維持）
3. **記号除去**: 今回のサンプルでは記号が含まれるタイトル（セクシー・セクシー・ダイナマイト）は raw で完全一致したため除去不要と判定。除去は行わない。
4. **末尾装飾剥離**: `[...]`, `(...)` をループで除去する。今回の観測では `-...-` は未出現だが仕様として維持する。

**更新点**: `normalized` バリアントは今回のサンプルでは raw と差がなかった。実装では「raw → stripped → 採用なし」の2段階で十分（normalized は raw と同等のため separate 試行の価値が低い）。ただし NFKC 正規化は実装上のリスクが低いため適用は維持する。

## スコア配点最終案

今回のサンプルから分かった重要な観察:

1. **title 完全一致が最重要**: raw でのヒットはすべて title の完全一致（4件）。誤紐付けは0件。
2. **stripped title ヒット時の artist 確認が必須**: stripped でのヒット（3件）は title が剥離後のみ一致するため、artist 一致を追加スコアで確認しないと誤紐付けリスクがある。
   - `Aqua Regia Squall [F]` → stripped で "Aqua Regia Squall" → artist "DJ owl-light vs xi" 完全一致 → 採用妥当
   - `Apocaliptix [Doomsayer]` → stripped で "Apocaliptix" → artist "Juggernaut." 完全一致 → 採用妥当
   - `ENTANGLEMENT(EX)` → stripped で "ENTANGLEMENT" → BMS側 artist "ABE3" vs DB artist "ABE3/さ" → 部分一致 → 採用の検討余地あり（主要アーティスト名は一致）
   - `energy night [Uchither]` → stripped で "energy night" → BMS側 artist "KAH (BGA:自動車原付 イラスト:手鞠鈴)" vs DB artist "KAH　(BGA:自動車原付　イラスト:手鞠鈴) obj;nytou" → ほぼ一致（全半角・obj付記の差のみ）→ 採用妥当だが BMS 側のデータが別バージョン（objファイル作者を除いた登録）
3. **Halcyon のケース**: stripped で "Halcyon" → 2件ヒット。1件は "Halcyon -MY INTERPRETATION-"（artist: Remixed by BACO）、もう1件は "Halcyon"（artist: xi / yama_ko）。DB artist は "xi"。2件目が artist "xi" を含むが、artist が "xi / yama_ko" と完全一致でない。同点首位の場合（title 部分一致で同スコア）は採用しないルールが適切。

初期案のスコア配点をそのまま採用する。

| 項目 | 配点 | 備考 |
|---|---|---|
| title 完全一致 | +60 | 最重要 |
| title 正規化後完全一致 | +50 | 揺れ吸収（記号・空白・大小・全半角） |
| title 部分一致 | +25 | 弱い手がかり |
| artist 完全一致 | +20 | アーティスト一致は表記揺れが多いため上限を低く |
| artist 正規化後完全一致 | +15 | 同上 |
| artist トークン共通率 × 10 | 0〜+10 | feat./BGI 等の表記揺れに弱く対応 |

**スコア計算ルール（初期案を維持）**

- title 系3項目から最高スコア1項目のみ採用（排他、最大+60）
- artist 系2項目から最高スコア1項目のみ採用（排他、最大+20）
- artist トークン共通率は独立に加算（最大+10）
- 合計最大スコア: 60 + 20 + 10 = **90点**

## 閾値最終値

- **採用閾値: 50（初期値から変更なし）**
- 同点首位が複数あった場合も採用しない（曖昧）

**根拠**: title 完全一致（+60）または title 正規化後完全一致（+50）が閾値50を超える。
stripped で title 部分一致（+25）のみの場合は採用しない（例: Halcyon 2件で同点の場合）。
artist 情報も部分的に加算されるケースでは閾値を超えうるが、artist トークン共通率が低い場合は保護される。

## top1 採用率と誤紐付け率の見積もり

**目視確認結果（raw クエリベース）**

- 昇順5件: 2件で count=1（title 完全一致 → top1 採用確実）
- 降順5件: 2件で count=1（title 完全一致 → top1 採用確実）
- 合計10件中4件が raw クエリで完全一致 → 採用率 40%（raw のみ）

**stripped クエリまで加えた場合**

- さらに4件でヒット（Aqua Regia Squall, Apocaliptix, ENTANGLEMENT, energy night）
- うち `Aqua Regia Squall`, `Apocaliptix` は artist 完全一致 → 採用妥当（誤紐付けなし）
- `energy night` は artist が全半角差+obj情報差あり → artist トークン共通率で部分加算、閾値は超える見込み。ただしアーティスト付記情報（BGA作者等）の扱いが別バージョン登録な点は許容範囲
- `ENTANGLEMENT` は artist が "ABE3" (BMS) vs "ABE3/さ" (DB) → トークン共通率で部分加算。閾値到達するか微妙だが、目視では採用妥当

**採用率見積もり（スコアリング適用後）**: 10件中7〜8件で采用確実または採用妥当（70〜80%）
**誤紐付け率**: 0件（全サンプルで明らかな誤紐付けなし）

**ヒット0件のサンプル**

- `After School Dessert(Uzawa&Kazusa HardCore Remix)[SP Semla]`: 3バリアント全てでヒット0。このようなリミックス主体タイトルはそもそも BMS Search に登録されていない（または別曲として未登録）。未紐付けのまま扱う。

## 結論

- 正規化ルールは初期案に沿って実装。ただし `normalized` バリアントは `raw` とほぼ等価で単独試行の効果が低いため、実装では「raw 試行 → stripped 試行 → 採用なし」の2段階とする。NFKC 正規化は両バリアントに適用。
- スコア配点・閾値は初期案（閾値50）をそのまま採用。サンプルの範囲では誤紐付けは観測されなかった。
- 末尾の `[...]`, `(...)` 剥離は有効。`-...-` は今回未出現だが仕様として維持。
- artist 一致がスコアリングの重要なセーフガードになる（title 部分一致のみでは閾値50に届かない）。

設計ドキュメントの「フォールバック検索の正規化・スコアリング」セクションをこの結論で更新する。
