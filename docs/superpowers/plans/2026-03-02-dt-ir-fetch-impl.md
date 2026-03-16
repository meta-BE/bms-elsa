# 難易度表IR一括取得 + chart_meta PK変更 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 難易度表ビューから未導入譜面を含む全エントリのLR2IR情報を一括取得できるようにする

**Architecture:** chart_metaのPKを(md5,sha256)→md5のみに変更し、BulkFetchIRUseCaseをmd5リスト受け取り型に汎用化。IRHandlerにmetaRepo依存を追加して、ChartListView/DifficultyTableViewの両方から呼べるようにする。

**Tech Stack:** Go (SQLite, Wails v2), Svelte (TypeScript, TanStack Table)

---

## Task 1: スキーマ変更 + ドメインモデル更新

**Files:**
- Modify: `internal/adapter/persistence/migrations.go`
- Modify: `internal/domain/model/song.go`
- Modify: `internal/domain/model/repository.go`

**Step 1: migrations.goにchart_metaマイグレーションを追加**

`RunMigrations`の末尾（イベントマッピングシード投入の後）に追加:

```go
	// chart_meta: (md5, sha256) UNIQUE → md5 UNIQUE に変更
	var hasOldSchema int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chart_meta') WHERE name='sha256'`).Scan(&hasOldSchema)
	if hasOldSchema > 0 {
		// 旧スキーマの場合のみマイグレーション
		var hasMD5Unique int
		_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_index_list('chart_meta') WHERE origin='u'`).Scan(&hasMD5Unique)
		// UNIQUE(md5,sha256)が存在するか確認
		row := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='chart_meta'`)
		var ddl string
		_ = row.Scan(&ddl)
		if strings.Contains(ddl, "UNIQUE(md5, sha256)") {
			if _, err := db.Exec(`
				CREATE TABLE chart_meta_new (
					id               INTEGER PRIMARY KEY AUTOINCREMENT,
					md5              TEXT NOT NULL UNIQUE,
					sha256           TEXT NOT NULL DEFAULT '',
					lr2ir_tags       TEXT,
					lr2ir_body_url   TEXT,
					lr2ir_diff_url   TEXT,
					lr2ir_notes      TEXT,
					lr2ir_fetched_at TEXT,
					working_body_url TEXT,
					working_diff_url TEXT,
					created_at       TEXT NOT NULL DEFAULT (datetime('now')),
					updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
				);
				INSERT OR IGNORE INTO chart_meta_new
					(md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes,
					 lr2ir_fetched_at, working_body_url, working_diff_url, created_at, updated_at)
				SELECT md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes,
					lr2ir_fetched_at, working_body_url, working_diff_url, created_at, updated_at
				FROM chart_meta
				GROUP BY md5
				HAVING id = MAX(id);
				DROP TABLE chart_meta;
				ALTER TABLE chart_meta_new RENAME TO chart_meta;
			`); err != nil {
				return fmt.Errorf("chart_meta migration: %w", err)
			}
		}
	}
```

また、既存の`CREATE TABLE IF NOT EXISTS chart_meta`定義を新スキーマに更新（新規DB用）:

```sql
CREATE TABLE IF NOT EXISTS chart_meta (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    md5              TEXT NOT NULL UNIQUE,
    sha256           TEXT NOT NULL DEFAULT '',
    lr2ir_tags       TEXT,
    lr2ir_body_url   TEXT,
    lr2ir_diff_url   TEXT,
    lr2ir_notes      TEXT,
    lr2ir_fetched_at TEXT,
    working_body_url TEXT,
    working_diff_url TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
)
```

旧スキーマの`idx_chart_meta_md5`と`idx_chart_meta_sha256`のCREATE INDEX文は削除（md5はUNIQUE制約でカバー）。

**Step 2: song.goからChartKey型を削除**

`internal/domain/model/song.go:88-92` の `ChartKey` struct を削除。

**Step 3: repository.goのMetaRepositoryインターフェースを更新**

```go
type MetaRepository interface {
	GetSongMeta(ctx context.Context, folderHash string) (*SongMeta, error)
	UpsertSongMeta(ctx context.Context, meta SongMeta) error
	GetChartMeta(ctx context.Context, md5 string) (*ChartIRMeta, error)
	UpsertChartMeta(ctx context.Context, meta ChartIRMeta) error
	BulkUpsertChartMeta(ctx context.Context, metas []ChartIRMeta) error
	UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error
	ListEventMappings(ctx context.Context) ([]EventMapping, error)
	UpsertEventMapping(ctx context.Context, m EventMapping) error
	DeleteEventMapping(ctx context.Context, id int) error
	ListUnsetSongsWithIRURLs(ctx context.Context) ([]SongIRURLs, error)
	// IR未取得の譜面md5一覧（songdata.songベース）
	ListUnfetchedChartMD5s(ctx context.Context) ([]string, error)
	// 難易度表の未取得エントリmd5一覧
	ListUnfetchedDTEntryMD5s(ctx context.Context, tableID int) ([]string, error)
}
```

変更点:
- `GetChartMeta` — sha256パラメータ削除
- `UpdateWorkingURLs` — sha256パラメータ削除
- `ListUnfetchedChartKeys` → `ListUnfetchedChartMD5s`、戻り値 `[]ChartKey` → `[]string`
- `ListUnfetchedDTEntryMD5s` を新規追加

**Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: コンパイルエラー（インターフェース未実装）。これはTask 2で修正する。

**Step 5: コミット**

```bash
git add internal/adapter/persistence/migrations.go internal/domain/model/song.go internal/domain/model/repository.go
git commit -m "refactor: chart_metaのPKをmd5のみに変更、ChartKey削除、MetaRepositoryインターフェース更新"
```

---

## Task 2: Repository層の実装更新

**Files:**
- Modify: `internal/adapter/persistence/elsa_repository.go`

**Step 1: GetChartMetaを更新**

sha256パラメータを削除、WHERE条件をmd5のみに:

```go
func (r *ElsaRepository) GetChartMeta(ctx context.Context, md5 string) (*ChartIRMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes,
		        lr2ir_fetched_at, working_body_url, working_diff_url
		 FROM chart_meta WHERE md5 = ?`,
		md5,
	)
	// ... Scan部分は変更なし
}
```

**Step 2: UpsertChartMetaを更新**

`ON CONFLICT(md5, sha256)` → `ON CONFLICT(md5)`:

```go
func (r *ElsaRepository) UpsertChartMeta(ctx context.Context, meta model.ChartIRMeta) error {
	tagsStr := strings.Join(meta.Tags, ",")
	var fetchedAtStr *string
	if meta.FetchedAt != nil {
		s := meta.FetchedAt.UTC().Format(timeLayout)
		fetchedAtStr = &s
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO chart_meta (md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url, lr2ir_notes, lr2ir_fetched_at, working_body_url, working_diff_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(md5) DO UPDATE SET
		   sha256           = COALESCE(NULLIF(excluded.sha256, ''), chart_meta.sha256),
		   lr2ir_tags       = excluded.lr2ir_tags,
		   lr2ir_body_url   = excluded.lr2ir_body_url,
		   lr2ir_diff_url   = excluded.lr2ir_diff_url,
		   lr2ir_notes      = excluded.lr2ir_notes,
		   lr2ir_fetched_at = excluded.lr2ir_fetched_at,
		   working_body_url = COALESCE(NULLIF(excluded.working_body_url, ''), chart_meta.working_body_url),
		   working_diff_url = COALESCE(NULLIF(excluded.working_diff_url, ''), chart_meta.working_diff_url),
		   updated_at       = datetime('now')`,
		meta.MD5, meta.SHA256, tagsStr,
		meta.LR2IRBodyURL, meta.LR2IRDiffURL, meta.LR2IRNotes,
		fetchedAtStr, meta.WorkingBodyURL, meta.WorkingDiffURL,
	)
	return err
}
```

注意: sha256もCOALESCE/NULLIFパターンで保護する。未導入譜面からのBulkFetchで空sha256が来ても、既存の値を保持する。

**Step 3: BulkUpsertChartMetaを更新**

`ON CONFLICT(md5, sha256)` → `ON CONFLICT(md5)` に変更（UpsertChartMetaと同じパターン）。

**Step 4: UpdateWorkingURLsを更新**

sha256パラメータを削除:

```go
func (r *ElsaRepository) UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE chart_meta SET
			working_body_url = ?,
			working_diff_url = ?,
			updated_at = datetime('now')
		 WHERE md5 = ?`,
		workingBodyURL, workingDiffURL, md5,
	)
	return err
}
```

**Step 5: ListUnfetchedChartKeysをListUnfetchedChartMD5sにリネーム**

戻り値を`[]model.ChartKey` → `[]string`に変更:

```go
func (r *ElsaRepository) ListUnfetchedChartMD5s(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.md5
		FROM songdata.song s
		LEFT JOIN chart_meta cm ON s.md5 = cm.md5
		WHERE cm.id IS NULL OR cm.lr2ir_fetched_at IS NULL
		ORDER BY s.md5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var md5s []string
	for rows.Next() {
		var md5 string
		if err := rows.Scan(&md5); err != nil {
			return nil, err
		}
		md5s = append(md5s, md5)
	}
	return md5s, rows.Err()
}
```

**Step 6: ListUnfetchedDTEntryMD5sを新規追加**

```go
func (r *ElsaRepository) ListUnfetchedDTEntryMD5s(ctx context.Context, tableID int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT dte.md5
		FROM difficulty_table_entry dte
		LEFT JOIN chart_meta cm ON dte.md5 = cm.md5
		WHERE dte.table_id = ? AND (cm.id IS NULL OR cm.lr2ir_fetched_at IS NULL)
		ORDER BY dte.md5`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var md5s []string
	for rows.Next() {
		var md5 string
		if err := rows.Scan(&md5); err != nil {
			return nil, err
		}
		md5s = append(md5s, md5)
	}
	return md5s, rows.Err()
}
```

**Step 7: コミット**

```bash
git add internal/adapter/persistence/elsa_repository.go
git commit -m "refactor: elsa_repository.goをmd5のみPKに対応させ、ListUnfetchedDTEntryMD5sを追加"
```

---

## Task 3: Repository呼び出し元の更新

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`（2箇所: 行287, 行475）
- Modify: `internal/usecase/update_chart_meta.go`
- Modify: `internal/usecase/lookup_ir.go`（変更なし確認のみ）

**Step 1: songdata_reader.goのGetChartMeta呼び出しを更新**

行287付近（GetSongByFolder内）:
```go
// Before: irMeta, err := r.metaRepo.GetChartMeta(ctx, charts[i].MD5, charts[i].SHA256)
// After:
irMeta, err := r.metaRepo.GetChartMeta(ctx, charts[i].MD5)
```

行475付近（GetChartByMD5内）:
```go
// Before: irMeta, err := r.metaRepo.GetChartMeta(ctx, c.MD5, c.SHA256)
// After:
irMeta, err := r.metaRepo.GetChartMeta(ctx, c.MD5)
```

**Step 2: update_chart_meta.goを更新**

sha256パラメータを削除:

```go
func (u *UpdateChartMetaUseCase) Execute(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error {
	return u.metaRepo.UpdateWorkingURLs(ctx, md5, workingBodyURL, workingDiffURL)
}
```

**Step 3: lookup_ir.goは変更不要であることを確認**

`LookupIRUseCase.Execute(ctx, md5, sha256)` はsha256をChartIRMetaに格納するために使用。
UpsertChartMetaのシグネチャ（meta引数）は変わらないので変更不要。

**Step 4: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go internal/usecase/update_chart_meta.go
git commit -m "refactor: GetChartMeta/UpdateWorkingURLsの呼び出し元からsha256を削除"
```

---

## Task 4: BulkFetchIRUseCase リファクタ

**Files:**
- Modify: `internal/usecase/bulk_fetch_ir.go`

**Step 1: Execute をmd5リスト受け取り型に変更**

```go
func (u *BulkFetchIRUseCase) Execute(ctx context.Context, md5s []string, progressFn func(BulkFetchProgress)) (*BulkFetchResult, error) {
	result := &BulkFetchResult{Total: len(md5s)}

	for i, md5 := range md5s {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			return result, nil
		default:
		}

		resp, err := u.irClient.LookupByMD5(ctx, md5)
		if err != nil {
			if ctx.Err() != nil {
				result.Cancelled = true
				return result, nil
			}
			result.Failed++
			if progressFn != nil {
				progressFn(BulkFetchProgress{Current: i + 1, Total: len(md5s)})
			}
			continue
		}

		now := time.Now()
		meta := model.ChartIRMeta{
			MD5:       md5,
			FetchedAt: &now,
		}
		if resp.Registered {
			meta.Tags = resp.Tags
			meta.LR2IRBodyURL = resp.BodyURL
			meta.LR2IRDiffURL = resp.DiffURL
			meta.LR2IRNotes = resp.Notes
			result.Fetched++
		} else {
			result.NotFound++
		}

		if err := u.metaRepo.UpsertChartMeta(ctx, meta); err != nil {
			result.Failed++
		}

		if progressFn != nil {
			progressFn(BulkFetchProgress{Current: i + 1, Total: len(md5s)})
		}
	}

	return result, nil
}
```

変更点:
- 引数に `md5s []string` を追加
- `u.metaRepo.ListUnfetchedChartKeys(ctx)` の呼び出しを削除
- `key.MD5` → `md5`、`key.SHA256` を使わない

**Step 2: metaRepoフィールドが不要になったか確認**

`UpsertChartMeta` で使用しているのでmetaRepoは引き続き必要。ただし `ListUnfetchedChartKeys` は呼ばなくなるので、BulkFetchIRUseCaseの依存は `irClient` + `metaRepo`（UpsertChartMetaのみ使用）。

**Step 3: コミット**

```bash
git add internal/usecase/bulk_fetch_ir.go
git commit -m "refactor: BulkFetchIRUseCase.Executeをmd5リスト受け取り型に変更"
```

---

## Task 5: テスト更新

**Files:**
- Modify: `internal/usecase/bulk_fetch_ir_test.go`
- Modify: `internal/usecase/usecase_test.go`
- Modify: `internal/adapter/persistence/elsa_repository_test.go`

**Step 1: bulk_fetch_ir_test.goを更新**

mockMetaRepoForBulkから `unfetchedKeys` を削除し、テストが直接md5リストをExecuteに渡すように変更:

```go
type mockMetaRepoForBulk struct {
	model.MetaRepository
	upsertChartMetaCalls []model.ChartIRMeta
}

func (m *mockMetaRepoForBulk) UpsertChartMeta(_ context.Context, meta model.ChartIRMeta) error {
	m.upsertChartMetaCalls = append(m.upsertChartMetaCalls, meta)
	return nil
}
```

各テストのExecute呼び出しを変更:

```go
// TestBulkFetchIR_AllRegistered
md5s := []string{"aaa", "bbb"}
result, err := uc.Execute(context.Background(), md5s, func(p usecase.BulkFetchProgress) {
    progresses = append(progresses, p)
})

// TestBulkFetchIR_MixedResults
md5s := []string{"found", "notfound"}
result, err := uc.Execute(context.Background(), md5s, nil)

// TestBulkFetchIR_Cancellation
md5s := []string{"aaa", "bbb", "ccc"}
result, err := uc.Execute(ctx, md5s, func(p usecase.BulkFetchProgress) {
    if p.Current == 1 {
        cancel()
    }
})
```

**Step 2: usecase_test.goのmockMetaRepoを更新**

インターフェース変更に合わせてmock定義を更新:

```go
type mockMetaRepo struct {
	getSongMetaFunc              func(ctx context.Context, folderHash string) (*model.SongMeta, error)
	upsertSongMetaFunc           func(ctx context.Context, meta model.SongMeta) error
	getChartMetaFunc             func(ctx context.Context, md5 string) (*model.ChartIRMeta, error)
	upsertChartMetaFunc          func(ctx context.Context, meta model.ChartIRMeta) error
	bulkUpsertChartMetaFunc      func(ctx context.Context, metas []model.ChartIRMeta) error
	updateWorkingURLsFunc        func(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error
	listEventMappingsFunc        func(ctx context.Context) ([]model.EventMapping, error)
	upsertEventMappingFunc       func(ctx context.Context, m model.EventMapping) error
	deleteEventMappingFunc       func(ctx context.Context, id int) error
	listUnsetSongsWithIRURLsFunc func(ctx context.Context) ([]model.SongIRURLs, error)
}
```

メソッド実装の更新:
```go
func (m *mockMetaRepo) GetChartMeta(ctx context.Context, md5 string) (*model.ChartIRMeta, error) {
	return m.getChartMetaFunc(ctx, md5)
}

func (m *mockMetaRepo) UpdateWorkingURLs(ctx context.Context, md5, workingBodyURL, workingDiffURL string) error {
	return m.updateWorkingURLsFunc(ctx, md5, workingBodyURL, workingDiffURL)
}

// 旧ListUnfetchedChartKeys → ListUnfetchedChartMD5s
func (m *mockMetaRepo) ListUnfetchedChartMD5s(_ context.Context) ([]string, error) {
	return nil, nil
}

// 新規
func (m *mockMetaRepo) ListUnfetchedDTEntryMD5s(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}
```

TestUpdateChartMetaのテストも更新（sha256パラメータ削除）:
```go
func TestUpdateChartMeta(t *testing.T) {
	var calledMD5, calledBodyURL, calledDiffURL string
	repo := &mockMetaRepo{
		updateWorkingURLsFunc: func(_ context.Context, md5, workingBodyURL, workingDiffURL string) error {
			calledMD5 = md5
			calledBodyURL = workingBodyURL
			calledDiffURL = workingDiffURL
			return nil
		},
	}

	uc := usecase.NewUpdateChartMetaUseCase(repo)
	err := uc.Execute(context.Background(), "md5hash", "http://body.url", "http://diff.url")
	// ... assertions (calledSHA256の検証を削除)
}
```

**Step 3: elsa_repository_test.goを更新**

GetChartMeta呼び出しからsha256を削除:
```go
// 例: got, err := repo.GetChartMeta(ctx, "aaa", "bbb")
// →
got, err := repo.GetChartMeta(ctx, "aaa")
```

UpdateWorkingURLs呼び出しからsha256を削除:
```go
// 例: repo.UpdateWorkingURLs(ctx, "aaa", "bbb", "http://...", "http://...")
// →
repo.UpdateWorkingURLs(ctx, "aaa", "http://...", "http://...")
```

BulkUpsertChartMetaテストのGetChartMeta呼び出しも同様に更新。

**Step 4: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全テスト PASS

**Step 5: コミット**

```bash
git add internal/usecase/bulk_fetch_ir_test.go internal/usecase/usecase_test.go internal/adapter/persistence/elsa_repository_test.go
git commit -m "test: chart_meta PK変更とBulkFetchIRリファクタに合わせてテスト更新"
```

---

## Task 6: IRHandler更新 + app.go DI

**Files:**
- Modify: `internal/app/ir_handler.go`
- Modify: `app.go`

**Step 1: IRHandlerにmetaRepoを追加**

```go
type IRHandler struct {
	ctx         context.Context
	lookupIR    *usecase.LookupIRUseCase
	bulkFetchIR *usecase.BulkFetchIRUseCase
	updateChart *usecase.UpdateChartMetaUseCase
	metaRepo    model.MetaRepository

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewIRHandler(
	li *usecase.LookupIRUseCase,
	bf *usecase.BulkFetchIRUseCase,
	uc *usecase.UpdateChartMetaUseCase,
	mr model.MetaRepository,
) *IRHandler {
	return &IRHandler{lookupIR: li, bulkFetchIR: bf, updateChart: uc, metaRepo: mr}
}
```

**Step 2: UpdateChartMetaからsha256を削除**

```go
func (h *IRHandler) UpdateChartMeta(md5, workingBodyURL, workingDiffURL string) error {
	return h.updateChart.Execute(h.ctx, md5, workingBodyURL, workingDiffURL)
}
```

**Step 3: StartBulkFetchをリファクタ**

md5リストを取得してからExecuteに渡す:

```go
func (h *IRHandler) StartBulkFetch() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	// md5リスト取得（ロック外で実行）
	md5s, err := h.metaRepo.ListUnfetchedChartMD5s(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result, err := h.bulkFetchIR.Execute(ctx, md5s, func(p usecase.BulkFetchProgress) {
			wailsRuntime.EventsEmit(h.ctx, "ir:progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		doneData := map[string]interface{}{
			"cancelled": false,
			"error":     "",
		}
		if err != nil {
			doneData["error"] = err.Error()
		}
		if result != nil {
			doneData["total"] = result.Total
			doneData["fetched"] = result.Fetched
			doneData["notFound"] = result.NotFound
			doneData["failed"] = result.Failed
			doneData["cancelled"] = result.Cancelled
		}
		wailsRuntime.EventsEmit(h.ctx, "ir:done", doneData)
	}()

	return nil
}
```

**Step 4: StartDifficultyTableBulkFetchを新規追加**

StartBulkFetchとほぼ同じだが、md5リストの取得元が異なる:

```go
func (h *IRHandler) StartDifficultyTableBulkFetch(tableID int) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	md5s, err := h.metaRepo.ListUnfetchedDTEntryMD5s(h.ctx, tableID)
	if err != nil {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result, err := h.bulkFetchIR.Execute(ctx, md5s, func(p usecase.BulkFetchProgress) {
			wailsRuntime.EventsEmit(h.ctx, "ir:progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		doneData := map[string]interface{}{
			"cancelled": false,
			"error":     "",
		}
		if err != nil {
			doneData["error"] = err.Error()
		}
		if result != nil {
			doneData["total"] = result.Total
			doneData["fetched"] = result.Fetched
			doneData["notFound"] = result.NotFound
			doneData["failed"] = result.Failed
			doneData["cancelled"] = result.Cancelled
		}
		wailsRuntime.EventsEmit(h.ctx, "ir:done", doneData)
	}()

	return nil
}
```

**Step 5: app.goのDIを更新**

NewIRHandlerにelsaRepoを追加:

```go
a.IRHandler = internalapp.NewIRHandler(lookupIR, bulkFetchIR, updateChartMeta, elsaRepo)
```

**Step 6: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全テスト PASS

**Step 7: コミット**

```bash
git add internal/app/ir_handler.go app.go
git commit -m "feat: IRHandlerにStartDifficultyTableBulkFetchを追加、metaRepo依存追加"
```

---

## Task 7: フロントエンド更新

**Files:**
- Modify: `frontend/src/DifficultyTableView.svelte`
- Modify: `frontend/src/ChartDetail.svelte`
- Modify: `frontend/src/EntryDetail.svelte`
- Modify: `frontend/src/SongDetail.svelte`

**Step 1: DifficultyTableView.svelteにIR一括取得UIを追加**

import追加:
```typescript
import { EventsOn } from '../wailsjs/runtime/runtime'
import { StartDifficultyTableBulkFetch, StopBulkFetch } from '../wailsjs/go/app/IRHandler'
```

onDestroyがまだimportされていなければ追加:
```typescript
import { onMount, onDestroy, createEventDispatcher } from 'svelte'
```

IR関連の状態変数を追加（scriptセクション、既存の変数の後に）:
```typescript
let irFetching = false
let irProgress = { current: 0, total: 0 }
let irDoneMessage = ''
let irDoneTimer: ReturnType<typeof setTimeout> | null = null

function startBulkFetch() {
  if (!selectedTableId) return
  irFetching = true
  irProgress = { current: 0, total: 0 }
  irDoneMessage = ''
  if (irDoneTimer) { clearTimeout(irDoneTimer); irDoneTimer = null }
  StartDifficultyTableBulkFetch(selectedTableId).catch((e: Error) => {
    console.error('[IR] StartDifficultyTableBulkFetch failed:', e)
    irFetching = false
  })
}

function stopBulkFetch() {
  StopBulkFetch()
}
```

onMountにイベントリスナー追加:
```typescript
let offProgress: (() => void) | null = null
let offDone: (() => void) | null = null

onMount(() => {
  offProgress = EventsOn('ir:progress', (data: { current: number; total: number }) => {
    irProgress = data
  })
  offDone = EventsOn('ir:done', (data: { total: number; fetched: number; notFound: number; failed: number; cancelled: boolean }) => {
    irFetching = false
    const parts: string[] = []
    if (data.total === 0) {
      irDoneMessage = '対象なし'
    } else {
      if (data.fetched > 0) parts.push(`${data.fetched}件取得`)
      if (data.notFound > 0) parts.push(`${data.notFound}件未登録`)
      if (data.failed > 0) parts.push(`${data.failed}件失敗`)
      if (data.cancelled) parts.push('中断')
      irDoneMessage = parts.join(', ') || '完了'
    }
    irDoneTimer = setTimeout(() => { irDoneMessage = '' }, 5000)
  })

  // 既存の初期化処理（ListDifficultyTables等）はそのまま
})
```

onDestroyにクリーンアップ追加:
```typescript
onDestroy(() => {
  offProgress?.()
  offDone?.()
  if (irDoneTimer) clearTimeout(irDoneTimer)
})
```

テンプレートのヘッダーバーにIR取得UIを追加。`<SearchInput>` の前に配置:
```svelte
<div class="flex items-center gap-2">
  {#if irFetching}
    <span class="text-xs text-base-content/70">
      取得中: {irProgress.current.toLocaleString()} / {irProgress.total.toLocaleString()}
    </span>
    <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stopBulkFetch}>停止</button>
  {:else if irDoneMessage}
    <span class="text-xs text-success">{irDoneMessage}</span>
  {:else}
    <button class="btn btn-xs btn-outline" on:click|stopPropagation={startBulkFetch}>IR取得</button>
  {/if}
  <SearchInput bind:value={searchText} on:input={applyFilter} on:clear={applyFilter} />
</div>
```

**Step 2: UpdateChartMeta呼び出しからsha256を削除**

3つのファイルで同じ変更:

`ChartDetail.svelte:44`:
```typescript
// Before: await UpdateChartMeta(chart.md5, chart.sha256, editWorkingBodyUrl, editWorkingDiffUrl)
await UpdateChartMeta(chart.md5, editWorkingBodyUrl, editWorkingDiffUrl)
```

`EntryDetail.svelte:45`:
```typescript
await UpdateChartMeta(chart.md5, editWorkingBodyUrl, editWorkingDiffUrl)
```

`SongDetail.svelte:62`:
```typescript
await UpdateChartMeta(selectedChart.md5, editWorkingBodyUrl, editWorkingDiffUrl)
```

**Step 3: コミット**

```bash
git add frontend/src/DifficultyTableView.svelte frontend/src/ChartDetail.svelte frontend/src/EntryDetail.svelte frontend/src/SongDetail.svelte
git commit -m "feat: DifficultyTableViewにIR一括取得UI追加、UpdateChartMetaからsha256削除"
```

---

## Task 8: ビルド・統合テスト

**Step 1: 全テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全パッケージ PASS

**Step 2: Wailsビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: ビルド成功

**Step 3: 動作確認（手動）**

1. アプリ起動
2. 難易度表タブを選択
3. ドロップダウンで難易度表を選択
4. 「IR取得」ボタンをクリック
5. 進捗表示が更新されること
6. 完了メッセージが表示されること
7. ChartListViewタブの「IR取得」ボタンも引き続き動作すること
