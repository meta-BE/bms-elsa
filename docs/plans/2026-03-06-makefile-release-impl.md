# Makefile リリースコマンド 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** `make release-patch/minor/major` でセマンティックバージョンのタグを自動インクリメント・push し、GitHub Actions のビルドをトリガーする。

**Architecture:** Makefile に3つのターゲット (`release-patch`, `release-minor`, `release-major`) を定義。共通ロジックは内部ターゲット `_release` にまとめ、`BUMP` 変数で分岐する。

**Tech Stack:** GNU Make, シェルスクリプト (bash/zsh 互換), git

---

### Task 1: Makefile を作成

**Files:**
- Create: `Makefile`

**Step 1: Makefile を作成する**

```makefile
.PHONY: release-patch release-minor release-major _release

release-patch:
	@$(MAKE) _release BUMP=patch

release-minor:
	@$(MAKE) _release BUMP=minor

release-major:
	@$(MAKE) _release BUMP=major

_release:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "エラー: 未コミットの変更があります。先にコミットしてください。"; \
		exit 1; \
	fi
	@LATEST=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo $$LATEST | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo $$LATEST | sed 's/^v//' | cut -d. -f2); \
	PATCH=$$(echo $$LATEST | sed 's/^v//' | cut -d. -f3); \
	if [ "$(BUMP)" = "patch" ]; then \
		PATCH=$$((PATCH + 1)); \
	elif [ "$(BUMP)" = "minor" ]; then \
		MINOR=$$((MINOR + 1)); \
		PATCH=0; \
	elif [ "$(BUMP)" = "major" ]; then \
		MAJOR=$$((MAJOR + 1)); \
		MINOR=0; \
		PATCH=0; \
	fi; \
	NEW_VERSION="v$$MAJOR.$$MINOR.$$PATCH"; \
	echo "$$LATEST → $$NEW_VERSION"; \
	printf "リリースしますか？ [y/N] "; \
	read CONFIRM; \
	if [ "$$CONFIRM" = "y" ] || [ "$$CONFIRM" = "Y" ]; then \
		git tag $$NEW_VERSION && \
		git push origin $$NEW_VERSION && \
		echo "$$NEW_VERSION をpushしました。GitHub Actionsでビルドが開始されます。"; \
	else \
		echo "キャンセルしました。"; \
	fi
```

**Step 2: 動作確認（dry-run）**

未コミット変更がない状態で以下を実行し、バージョン計算と確認プロンプトが正しく表示されることを確認する。プロンプトで `N` を入力してキャンセル。

```bash
make release-patch
# 期待出力: v0.0.1 → v0.0.2
#           リリースしますか？ [y/N]

make release-minor
# 期待出力: v0.0.1 → v0.1.0

make release-major
# 期待出力: v0.0.1 → v1.0.0
```

未コミット変更がある状態でエラーになることも確認する。

**Step 3: コミット**

```bash
git add Makefile
git commit -m "ci: リリース用Makefileを追加"
```
