# GitHub Actions Windowsビルド 設計

## 目的

タグpush時にWindows向け `.exe` を自動ビルドし、GitHub Releaseに添付する。

## 制約

- プライベートリポジトリの無料枠（2,000分/月、Windows=2倍消費）に収める
- 自動実行はしない（タグpush時のみ）

## 設計

### ワークフロー

- **ファイル:** `.github/workflows/build-windows.yml`
- **トリガー:** `v*` パターンのタグpush
- **ランナー:** `windows-latest`
- **成果物:** `bms-elsa.exe` → GitHub Releaseに自動添付

### ビルドステップ

1. チェックアウト
2. Go 1.24 セットアップ
3. Node.js セットアップ
4. Wails CLI インストール（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）
5. `wails build -platform windows/amd64`
6. GitHub Release作成 + exe アップロード（`softprops/action-gh-release`）

### アプローチ

Wails CLIを直接インストールして `wails build` を実行する方式を採用。ローカル開発と同じビルドフローで一貫性があり、保守しやすい。

### 無料枠の見積もり

- Wailsビルド: 約5-10分 × 2倍(Windows) = 10-20分/回
- 月数回のリリースであれば無料枠内に十分収まる
