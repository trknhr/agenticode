# agenticode — 30-Day PoC Roadmap & Evaluation Sheet  
*“A self-driving coding agent you can ship, fork, or even sell.”*

---

## 0. ビジョン
> **agenticode** は **Go 製 CLI + Daemon** をコアとし、1 行コマンドで  
> - 実用的な React アプリを生成し  
> - GitHub に PR を発行し  
> - リポジトリ全体を要約できる  
> **汎用コーディングエージェント** を OSS (Apache-2.0) で公開するプロジェクトです。  
> 目標は *PoC 成功 → コミュニティ拡大 → 商用アドオン → M&A 可能性* まで射程に入れること。

---

## 1. 技術スタック（確定）

| レイヤ                       | 採用技術 / ライブラリ           | 理由 |
|------------------------------|---------------------------------|------|
| **CLI**                      | `spf13/cobra`, `spf13/viper`    | 定番・補完/設定が楽 |
| **デーモン & サーバ**        | Go net/http + **gRPC (buf)**    | MCP に親和性、プラグイン拡張容易 |
| **LLM クライアント**         | `go-openai`, `ollama-go`        | クラウド / ローカル切替 |
| **ログ**                     | `zerolog`                       | 高速・JSON 対応 |
| **ツール実行 (初期)**        | `write_file`, `apply_patch`, `run_shell` |
| **Git 操作**                 | `go-git` + `github.com/google/go-github` |
| **パッケージング**           | **goreleaser** + static binary |
| **ライセンス**               | **Apache-2.0** (OSS → 商用両立) |

> *TypeScript SDK / VS Code 拡張は PoC 後の v0.2 で着手。*

---

## 2. PoC Definition of Done (DOD)

| # | 機能                                | 合格条件 |
|---|-------------------------------------|----------|
| 1 | **React アプリ自動生成**            | `agenticode new "todo app"` ➜ `/ui` が生成され、`npm run dev` で CRUD が動く |
| 2 | **GitHub PR 自動作成**              | `agenticode propose "add search bar"` ➜ ブランチ作成・コミット・PR 発行・CI パス |
| 3 | **リポジトリ内容の自動説明**        | `agenticode explain` ➜ `docs/overview.md` を生成／更新（1k files < 30 s） |

### 評価スコア（合格ライン 70 % 以上）

| 重み | 項目              | 満点 |
|------|------------------|------|
| 30 % | 生成 UI 完成度    | 5 |
| 25 % | PR 品質          | 5 |
| 25 % | 説明精度          | 5 |
| 10 % | パフォーマンス    | 5 |
| 10 % | DX (CLI UX)      | 5 |

---

## 3. 30 Day Sprint Plan

| 期間      | マイルストーン & 主要タスク |
|-----------|-----------------------------|
| **Day 1** | リポジトリ作成 (`agenticode`)、LICENSE/README、GitHub Projects で「MVP v0.1」ボード |
| **Week 1**<br>Skeleton | - `cobra init` で CLI 雛形<br>- `/daemon` に gRPC+HTTP サーバ＋pprof<br>- `config.yaml` + CI (`go vet`, `go test`) |
| **Week 2**<br>Core loop | - Agent インターフェイス実装<br>- LLM クライアント (OpenAI/Ollama) ストリーミング対応<br>- MCP 風ツール登録 & 3 基本ツール |
| **Week 3**<br>Git & Guard | - `propose` コマンド：go-git & GitHub API 連携<br>- Playwright E2E テスト自動生成<br>- パス制御 / diff 確認フラグ |
| **Week 4**<br>Polish & Release | - `explain`：go-git → embedding → markdown 出力<br>- GoReleaser: darwin/linux/windows バイナリ<br>- README クイックスタート・アーキ図<br>- GitHub Release v0.1.0 & SNS 発信 |

---

## 4. 次フェーズ (v0.2+) スケッチ

- **TypeScript SDK + VS Code 拡張**（npm 配布）
- **Helm Chart / air-gapped bundle**（金融・公共導入向け）
- **Marketplace**: MCP ツール公開サイト & 収益分配
- **Team SaaS** : 履歴共有、SAML/SCIM、課金

---

## 5. 今日やること

1. GitHub リポジトリを `agenticode` で作成  
2. `go mod init github.com/your-handle/agenticode`  
3. `cobra init --pkg-name github.com/your-handle/agenticode`  
4. Issues を以下で切る：`#1 skeleton`, `#2 daemon`, `#3 new`, `#4 propose`, `#5 explain`  
5. 明日までに **Skeleton build green** を目標！

---

> 👊 **agenticode** starts now.  
> わからないこと・設計レビューが必要なときはいつでも声をかけてください。  
> Enjoy the sprint!
