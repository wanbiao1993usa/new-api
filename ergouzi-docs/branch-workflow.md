# Ergouzi 分支协作说明

> 更新日期：2026-05-26

这份文档给直接在 `new-api` 仓库里开发的 Ergouzi 组员看。

## 发生了什么变化

现在 `main` 已经调整为 Ergouzi 的二开主分支。

之前的 `main` 是较早的 New API / fork 分支，已经不再代表 Ergouzi 当前实际使用的开发线。
历史二开分支 `codex/subscription-redemption-codes` 已经被快进合入到 `main`。

当前分支职责如下：

| 分支 | 用途 |
|---|---|
| `main` | Ergouzi 二开主分支。日常开发从这里开始。 |
| `upstream-sync` | 跟踪上游 New API 的 `main`。不要在这里提交 Ergouzi 业务改动。 |
| `codex/subscription-redemption-codes` | 历史二开分支。目前和 `main` 指向同一个提交，但后续不要再基于它开发。 |
| `feature/*` / `fix/*` | 日常功能分支和修复分支，从 `main` 拉出。 |
| `sync/upstream-YYYYMMDD` | 同步上游 New API 时使用的临时集成分支。 |

当前记录的参考点位：

| Ref | Commit |
|---|---|
| `origin/main` | `c1c0356a3abe702da498d5d2472ccb27a0e45027` |
| `origin/codex/subscription-redemption-codes` | `c1c0356a3abe702da498d5d2472ccb27a0e45027` |
| `origin/upstream-sync` | `65f8afe92276203a33413a1bd5d5172ccd46e04e` |

## 同步你的本地仓库

如果你本地没有未提交改动，可以直接执行：

```bash
git fetch origin --prune
git checkout main
git pull --ff-only origin main
```

拉取共享的上游基线分支：

```bash
git fetch origin upstream-sync:upstream-sync
git branch --set-upstream-to=origin/upstream-sync upstream-sync
```

新需求或修复请从 `main` 拉分支：

```bash
git checkout main
git pull --ff-only origin main
git checkout -b feature/your-feature
```

## 如果你还停在 `codex/subscription-redemption-codes`

先检查本地状态：

```bash
git status --short --branch
```

如果工作区是干净的：

```bash
git fetch origin --prune
git checkout main
git pull --ff-only origin main
```

如果你的本地 `main` 不存在，或者你不确定它是不是正确的：

```bash
git fetch origin --prune
git checkout -B main origin/main
```

## 如果你本地有未提交改动

不要直接切分支。建议先 stash：

```bash
git status --short --branch
git stash push -u -m "before-main-sync"
git fetch origin --prune
git checkout main
git pull --ff-only origin main
git stash pop
```

如果 `stash pop` 出现冲突，先解决冲突再继续。不要把未解决的冲突标记提交进去。

## 上游同步规则

大多数开发同学不需要配置官方上游 remote。平时只需要使用 `origin/upstream-sync` 作为共享的上游基线。

负责同步上游的人可以配置：

```bash
git remote add upstream https://github.com/QuantumNous/new-api.git
git remote set-url --push upstream DISABLED
git fetch upstream main
```

规则：

- 不要在 `upstream-sync` 上提交 Ergouzi 业务代码。
- 不要 push 到官方上游仓库。
- 保持 `upstream-sync` 是干净的上游跟踪分支。
- 同步上游时，从 Ergouzi `main` 拉一个临时分支：

```bash
git checkout main
git pull --ff-only origin main
git checkout -b sync/upstream-YYYYMMDD
git fetch upstream main
git merge upstream/main
```

同步上游的 PR 里建议记录：

- 上游同步的起止 commit；
- Ergouzi `main` 的基准 commit；
- 冲突文件和解决说明；
- 数据库或配置变更；
- 验证结果；
- 是否建议部署。

## 不要做这些事

- 不要再从 `codex/subscription-redemption-codes` 开始新开发。
- 不要把 Ergouzi 业务改动提交到 `upstream-sync`。
- 不要 force-push `main`。
- 不要绕过同步分支和验证，直接把上游改动合进 `main`。
- 不要 push 到 `upstream`，保持它的 push URL 禁用。

## 不确定时怎么处理

先停下来，把下面信息贴给维护同学：

```bash
git status --short --branch
git branch -vv
git remote -v
```

执行 `reset`、`rebase`、`push --force` 或删除分支前，先确认清楚。
