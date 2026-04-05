# 重複検知スキャンへのMD5完全一致検出の追加 設計書

## 概要

現在の重複検知スキャンはWAV定義MinHash+ファジーマッチングのみだが、本来メインとなるべき「同一MD5が複数フォルダに存在する」ケースの検出が含まれていない。MD5完全一致による確定的な重複検出を第1段階として追加し、ファジーマッチングは第2段階として残す。

## 方針

2段階パイプラインで実装する。

1. **第1段階（MD5完全一致）**: 同一MD5が2つ以上のフォルダに存在するペアを抽出 → Union-Findに投入（スコア100%）
2. **第2段階（ファジーマッチ）**: 第1段階で既にペアリング済みのフォルダ対を除外し、残りに対して既存のファジーマッチを実行 → 同じUnion-Findに投入

統合されたグループリストを返す。MD5一致で確定したペアにはファジースコア計算を行わない。

## 変更箇所

### 1. モデル（`internal/domain/model/repository.go`）

`MD5DuplicatePair` 構造体を追加：

```go
type MD5DuplicatePair struct {
    FolderA string
    FolderB string
    MD5     string
}
```

`SongRepository` インターフェースにメソッド追加：

```go
ListMD5DuplicateFolders(ctx context.Context) ([]MD5DuplicatePair, error)
```

### 2. データ取得（`internal/adapter/persistence/songdata_reader.go`）

`ListMD5DuplicateFolders` を実装：

```sql
SELECT s1.folder AS folder_a, s2.folder AS folder_b, s1.md5
FROM songdata.song s1
JOIN songdata.song s2 ON s1.md5 = s2.md5 AND s1.folder < s2.folder
WHERE s1.md5 IS NOT NULL AND s1.md5 != ''
```

同一MD5を共有するフォルダのペアを返す。`s1.folder < s2.folder` でペアの重複を排除。

### 3. グルーピングロジック（`internal/domain/similarity/grouping.go`）

#### `DuplicateMember` にフィールド追加

```go
type DuplicateMember struct {
    SongInfo
    Scores   ScoreResult
    MD5Match bool // MD5完全一致で検出されたか
}
```

#### `FindDuplicateGroups` のシグネチャ変更

```go
func FindDuplicateGroups(songs []SongInfo, md5Pairs []MD5DuplicatePair, threshold int) []DuplicateGroup
```

引数に `md5Pairs` を追加。内部に `MD5DuplicatePair` の型定義は不要で、`model.MD5DuplicatePair` を使うとドメイン→モデルの依存が生じるため、ここではフォルダハッシュのペアのスライス型を受け取る形とする：

```go
type FolderPair struct {
    FolderA string
    FolderB string
}

func FindDuplicateGroups(songs []SongInfo, md5Pairs []FolderPair, threshold int) []DuplicateGroup
```

ユースケース層で `MD5DuplicatePair` → `FolderPair` への変換を行う。

#### ロジック

1. `songs` からFolderHash→indexのマップを構築
2. `md5Pairs` を走査し、各ペアのフォルダをUnion-Findで統合。`bestScore` にTotal=100のScoreResultを設定、`md5Matched` セットにペアを記録
3. 既存のブロッキング→ペア比較を実行。ただし `md5Matched` に含まれるペアはスキップ
4. 以降のグループ集約は従来通り。`MD5Match` フラグは `md5Matched` セットから判定

### 4. ユースケース（`internal/usecase/scan_duplicates.go`）

`Execute` メソッドの変更：

```go
func (u *ScanDuplicatesUseCase) Execute(ctx context.Context) ([]similarity.DuplicateGroup, error) {
    // 1. MD5ペア取得
    md5Pairs, err := u.songRepo.ListMD5DuplicateFolders(ctx)
    // ...

    // 2. フォルダ一覧取得（従来通り）
    songGroups, err := u.songRepo.ListSongGroupsForDuplicateScan(ctx)
    // ...

    // 3. MD5DuplicatePair → FolderPair に変換
    folderPairs := make([]similarity.FolderPair, len(md5Pairs))
    for i, p := range md5Pairs {
        folderPairs[i] = similarity.FolderPair{FolderA: p.FolderA, FolderB: p.FolderB}
    }

    // 4. 2段階パイプラインで重複グループ検出
    groups := similarity.FindDuplicateGroups(songs, folderPairs, defaultThreshold)
    return groups, nil
}
```

### 5. フロントエンド

#### `DuplicateView.svelte`（一覧）

変更なし。MD5一致グループは `Score: 100` で表示される。

#### `DuplicateDetail.svelte`（詳細）

- グループ内に `MD5Match: true` のメンバーがいる場合、類似度内訳セクションの代わりに「MD5一致」と表示
- MD5一致メンバーとファジーマッチメンバーが混在するグループでは：
  - グループヘッダに「MD5一致あり」バッジを追加
  - 類似度内訳はファジーマッチ側のスコアを表示

### 6. 変更しないもの

- `similarity.go`（Score関数）
- `duplicate_handler.go`（シグネチャ変更なし、そのまま `scanDuplicates.Execute` を呼ぶ）
- マージ機能

## テスト

### `grouping_test.go`

- MD5ペアのみでグループ形成されるケース
- MD5ペア + ファジーマッチで1つのグループに統合されるケース（A-B MD5一致、B-Cファジー → {A, B, C}）
- MD5ペアがある場合にファジーマッチのスコア計算がスキップされることの確認
- `MD5Match` フラグが正しく設定されることの確認
