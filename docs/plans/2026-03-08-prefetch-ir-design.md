# LR2IR事前取得CLIコマンド 設計

## 目的

LR2IRに現存する全MD5（約33.6万件）のIR情報を事前取得し、初期化済みelsa.dbをビルドに同梱する。
ユーザーがアプリ起動直後からIR情報を参照できるようにするため。

## コマンドインターフェース

```
go run ./cmd/prefetch-ir \
  --db build/elsa.db \
  --csv cmd/prefetch-ir/bmsid-md5-map.csv \
  --interval 200ms \
  --start-id 150000
```

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `--db` | `build/elsa.db` | 出力先elsa.db（存在すれば再開、なければ新規作成+マイグレーション） |
| `--csv` | `cmd/prefetch-ir/bmsid-md5-map.csv` | bmsid,md5のCSVファイル |
| `--interval` | `200ms` | リクエスト間隔 |
| `--start-id` | `0`（先頭から） | 再開時のbmsid。この値以降のbmsidから処理開始 |

## 中断・再開

- **Ctrl+C（SIGINT）** で安全に停止し、最後に処理したbmsidを表示:
  ```
  中断しました。再開するには: --start-id 150234
  ```
- `--start-id` 指定でCSVの該当行から処理再開
- さらに、既にchart_metaにfetched_atがセットされているmd5は自動スキップ（二重取得防止）

## 進捗表示

```
[150234/336053] bmsid=150234 md5=abc123... registered=true (残り約10h23m)
```

- `\r`で1行上書き表示
- エラー時のみ改行で出力

## 処理フロー

1. CSV全行読み込み（bmsidの昇順前提）
2. `--start-id` 以降の行をフィルタ
3. elsa.dbを開く（なければ作成+マイグレーション実行）
4. 各md5について:
   - chart_metaにfetched_atがあればスキップ
   - `LR2IRClient.LookupByMD5` で取得
   - `ElsaRepository.UpsertChartMeta` で保存（登録/未登録どちらも）
   - interval待機
5. SIGINT受信で安全に停止、再開IDを表示

## ファイル構成

```
cmd/prefetch-ir/
  main.go                # CLIエントリーポイント
  bmsid-md5-map.csv      # MD5リスト（約33.6万行、リポジトリに含める）
build/
  elsa.db                # 生成済みDB（リポジトリにコミット）
```

## 既存コード再利用

- `gateway.LR2IRClient` — intervalを外部から設定可能にする小改修（`SetInterval(d time.Duration)`メソッド追加）
- `persistence.RunMigrations(db)` — スキーマ作成
- `persistence.NewElsaRepository(db)` — UpsertChartMeta

## CI変更

`build-windows.yml` で `build/elsa.db` を `build/bin/` にコピーしてzipに含める。

## 制約・注意事項

- 一回きりの実行を想定。定期更新は行わない
- LR2IRへの負荷を考慮し、デフォルト200ms間隔（5回/秒）
- 33.6万件 × 200ms ≈ 約18.7時間
