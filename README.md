# agenticode — Natural-Language Coding PoC (30-Day Plan)

---

## 0. ビジョン（フォーカスを再定義）
**agenticode** は **“自然言語でコードを書き始められる”** 体験を最速で実証する  
Go 製の **単体 CLI** です（サーバ／デーモンは今回不要）。

1. 開発者は **１行の自然言語** で仕様を伝える  
2. agenticode が **LLM + ツール** でコード差分を生成  
3. ユーザー確認 → コード適用 → Git 操作まで完結  

---

## 1. 技術スタック（更新版）

| レイヤ            | 採用技術 / ライブラリ              | 理由 |
|-------------------|------------------------------------|------|
| **CLI**           | `spf13/cobra`, `spf13/viper`       | 定番・補完/設定が楽 |
| **LLM クライアント**| `go-openai`, `ollama-go`          | クラウド / ローカル切替 |
| **ツール実行**    | `write_file`, `apply_patch`, `run_shell` | 最小でコード生成ループ成立 |
| **Git 操作**      | `go-git`, `go-github`              | ブランチ作成・PR 発行 |
| **テスト実行**    | `npm test` など既存スクリプトをエージェントが呼ぶ | 自然言語→テスト追加まで確認 |
| **パッケージング**| `goreleaser`（static binary）      | インストール 1 コマンド |
| **ライセンス**    | Apache-2.0                          | OSS → 商用両立 |

> **サーバ／デーモンは今回スコープ外**。すべて 1 プロセスで完結。

---

## 2. PoC Definition of Done (DOD) ― *自然言語コーディング版*

| # | 機能                               | 合格条件 (30 日以内) |
|---|------------------------------------|----------------------|
| **1** | **自然言語⇄コード生成ループ** | `agenticode code "Create a React todo list with add/complete/delete"` 実行で<br>① 新規ファイル群を書き出し<br>② `npm run dev` がエラーなく起動<br>③ ユーザーが差分を確認 → `y` で適用 |
| **2** | **GitHub PR 自動作成**         | `agenticode propose "add search bar"` で<br> ブランチ作成 → コード生成 → Commit → PR 作成（CI 緑） |
| **3** | **リポジトリ内容の説明**       | `agenticode explain` で<br> `/docs/overview.md` を 30 秒以内に生成／更新 |

### 評価スコア

| 重み | 項目                | 満点 |
|------|--------------------|------|
| 35 % | 自然言語→コード精度 | 5 |
| 25 % | PR 品質            | 5 |
| 20 % | 説明精度            | 5 |
| 10 % | 操作体験 (UX)       | 5 |
| 10 % | パフォーマンス      | 5 |

**合格ライン：70 % 以上**

---

## 評価システム (Evaluation System)

agenticodeには、生成されたコードの品質を評価するための評価システムが実装されています。

### 評価コマンド

```bash
# 単一テストの実行
make eval

# 全テストの実行
make eval-all

# 詳細レポート付き
make eval-verbose

# JSON形式で結果を保存
make eval-report
```

### テストケース形式

テストケースはYAML形式で記述します（`tests/codegen/*.yaml`）:

```yaml
name: http-server
description: "Create a simple HTTP server in Go"
prompt: "Create a simple HTTP server in Go that listens on port 8080"

expect:
  files:
    - path: main.go
      should_contain:
        - "package main"
        - "http.ListenAndServe"
        - ":8080"

eval_mode: gpt  # or "static"
criteria:
  - "Does the code compile?"
  - "Is the server listening on port 8080?"
```

### 評価メトリクス

- ✅ **Pass率**: 期待条件を満たしたか
- 📁 **構成適合度**: ファイル構成の適切さ
- 🧠 **意図理解スコア**: プロンプトと生成コードの整合性
- 💡 **コード品質**: 可読性、命名、スタイル
- 🧪 **実行性**: ビルド・実行の成功

---

## 3. 30-Day Sprint Plan (サーバなし版)

| 期間      | マイルストーン & 主要タスク |
|-----------|-----------------------------|
| **Day 1** | Repo `agenticode` 作成・`cobra init`・README 雛形 |
| **Week 1** | - `code` コマンド骨格 (`agenticode code "…"`) <br> - LLM wrapper + streaming diff プレビュー <br> - `write_file` & `apply_patch` ツール |
| **Week 2** | - React Todo 生成の **プロンプトテンプレート** 固定化<br> - `npm run dev` コンパイル確認 & Playwright smoke test<br> - 差分確認 `--dry-run` オプション |
| **Week 3** | - `propose`：go-git ブランチ → GitHub API PR 作成<br> - 自動コミットメッセージ (LLM 要約) <br> - CI (GitHub Actions) が緑になるサンプル |
| **Week 4** | - `explain`：go-git 解析→ embeddings → overview.md 生成<br> - GoReleaser 設定 / バイナリ配布 (darwin/linux/windows)<br> - README に GIF デモ + リリース v0.1.0 |

---

## 4. 今日の To-Do

1. **リポジトリ & skeleton**  
   ```bash
   gh repo create agenticode --public
   go mod init github.com/your-handle/agenticode
   cobra init --pkg-name github.com/your-handle/agenticode
    ```

2. Issue 切り出し

#1 code コマンド MVP

#2 LLM patch generator

#3 propose GitHub PR

#4 explain repo overview

Day-1 Goal: agenticode code "hello world" が main.go を生成し go run で実行できるところまで。