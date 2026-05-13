# MaebrCode Repo Memory

## Identity
- MaebrCode is maintained by Mohammad Tihame.
- Go 1.24 terminal AI coding assistant with Cobra CLI, Bubble Tea TUI, SQLite persistence, tool-calling agents, and optional LSP/MCP integrations.
- This workspace copy is a source snapshot without a `.git` directory.

## Quick Commands
- Build: `go build -o maebrcode`
- Run TUI: `go run .`
- Run non-interactive prompt: `go run . -p "explain X" -f text|json`
- Run tests: `go test ./...`
- Generate config schema: `go run cmd/schema/main.go > maebrcode-schema.json`
- Snapshot build: `./scripts/snapshot`
- Release tag helper: `./scripts/release [--minor]`
- Inferred if SQL/query files change: `sqlc generate`

## Entry Flow
- `main.go` defers panic recovery and calls `cmd.Execute()`.
- `cmd/root.go` is the real entrypoint:
  - parses flags (`-d`, `-c`, `-p`, `-f`, `-q`, `-v`)
  - loads config
  - opens SQLite and runs migrations
  - creates `app.App`
  - initializes MCP tools in background
  - runs either non-interactive agent flow or Bubble Tea TUI
- Non-interactive mode auto-approves permissions for the created session and prints plain text or JSON via `internal/format/format.go`.

## Config And Memory Loading
- Config search order:
  - `$HOME/.maebrcode.json`
  - `$XDG_CONFIG_HOME/maebrcode/.maebrcode.json`
  - `$HOME/.config/maebrcode/.maebrcode.json`
  - local `./.maebrcode.json` merged on top
- Default data dir is `.maebrcode`.
- Default prompt context paths are declared in `internal/config/config.go`.
- Prompt context is loaded once per process via `sync.Once` in `internal/llm/prompt/prompt.go`; edits to memory/context files may require restart to affect prompts.
- Local `.maebrcode.json` includes `memory.md` in `contextPaths`, so future repo sessions auto-ingest this file.
- Provider default priority in config selection:
  - Copilot
  - Anthropic
  - OpenAI
  - Gemini
  - Groq
  - OpenRouter
  - XAI
  - Bedrock
  - Azure
  - VertexAI
- Watch out: `internal/config/config.go` defines `LSPConfig.Disabled` with `json:"enabled"` even though docs/examples talk about `disabled`.

## Persistence Model
- DB bootstrap: `internal/db/connect.go`
- Migrations: `internal/db/migrations/*`
- Core tables:
  - `sessions`: title, parent linkage, message counts, token counts, cost, `summary_message_id`
  - `messages`: role + JSON `parts` payload + model + finish timestamp
  - `files`: per-session file snapshots and versions
- SQL sources: `internal/db/sql/*.sql`
- Generated sqlc outputs:
  - `internal/db/db.go`
  - `internal/db/models.go`
  - `internal/db/querier.go`
  - `internal/db/*sql.go`
- Note: migration comments say timestamps are milliseconds, but schema uses `strftime('%s','now')`, so actual values are seconds.
- Note: `internal/db/sql/files.sql` contains `ListNewFiles` on `is_new`, but migrations do not create that column; looks stale/unused.

## Core Services
- `internal/app/app.go` wires:
  - `session.Service`
  - `message.Service`
  - `history.Service`
  - `permission.Service`
  - `agent.Service` for the coder agent
  - LSP client map
- All major services use `internal/pubsub` brokers so the TUI reacts to session/message/log/permission/agent events.

## Message And History Model
- Message parts live in `internal/message/content.go`:
  - reasoning
  - text
  - image URL
  - binary attachment
  - tool call
  - tool result
  - finish marker
- `internal/message/message.go` serializes/deserializes these parts for DB storage.
- `internal/history/file.go` stores initial file content plus later versions. Version sequencing is path-based and not isolated to one session when computing the next version label.

## Agent Architecture
- Main loop: `internal/llm/agent/agent.go`
- Providers created by `createAgentProvider()` using config-selected model/provider pairs.
- Separate providers exist for:
  - coder
  - task agent
  - title generation
  - summarization
- Title generation happens asynchronously on the first user message in a session.
- Summarization stores a summary message in the same session and sets `summary_message_id`.
- Important mismatch: README and UI text say compacting creates a new session, but code keeps the same session and trims usable history from the summary forward.
- MCP tool discovery/execution lives in `internal/llm/agent/mcp-tools.go`; discovered MCP tools are cached globally for the process.

## Tool Inventory
- Main coder agent tools are assembled in `internal/llm/agent/tools.go`:
  - `bash`
  - `edit`
  - `fetch`
  - `glob`
  - `grep`
  - `ls`
  - `sourcegraph`
  - `view`
  - `patch`
  - `write`
  - `agent`
  - MCP tools
  - `diagnostics` when LSP exists
- Task agent is read/search only:
  - `glob`
  - `grep`
  - `ls`
  - `sourcegraph`
  - `view`

## Tool Semantics That Matter
- `view`
  - reads text files with line numbers
  - 250 KB max size
  - default 2000 lines
  - records read time
  - appends file/project diagnostics when LSP is active
- `write`, `edit`, `patch`
  - enforce read-before-write checks using in-memory timestamps from `internal/llm/tools/file.go`
  - safety state resets when the process restarts
  - all update file history snapshots
- `patch`
  - uses custom patch grammar in `internal/diff/patch.go`
  - applies multi-file changes atomically after permission checks
- `bash`
  - uses a persistent shell per working directory in `internal/llm/tools/shell/shell.go`
  - default timeout `60000`
  - max timeout `600000`
  - output truncated to 30000 chars
  - some commands are banned up front
  - read-only safe commands skip permission prompts
- `fetch`
  - HTTP/HTTPS only
  - 5 MB max response
  - supports `text`, `markdown`, `html`
- `diagnostics`
  - only LSP capability exposed to the AI tool layer right now, even though the client supports much more
- `sourcegraph`
  - public code search via Sourcegraph GraphQL

## Permission Model
- Implemented in `internal/permission/permission.go`
- Requests are keyed by session/tool/action/path.
- Grants can be:
  - one-shot
  - persistent for the current session/path/tool/action
- Non-interactive mode auto-approves the whole session.

## LSP Architecture
- Startup hook: `internal/app/lsp.go`
- Core client: `internal/lsp/client.go`
- JSON-RPC transport: `internal/lsp/transport.go`
- Notification/request handlers: `internal/lsp/handlers.go`
- File watching/preloading: `internal/lsp/watcher/watcher.go`
- Workspace edit application: `internal/lsp/util/edit.go`
- Generated protocol bindings: `internal/lsp/protocol/*`
- Repo-local `.maebrcode.json` currently configures `gopls`.
- Watcher preloads high-priority files for some servers; for `gopls` it favors `go.mod`, `go.sum`, and `main.go`.

## TUI Map
- Root model: `internal/tui/tui.go`
- Pages:
  - chat: `internal/tui/page/chat.go`
  - logs: `internal/tui/page/logs.go`
- Chat components:
  - editor: `internal/tui/components/chat/editor.go`
  - message list/rendering: `internal/tui/components/chat/list.go`, `message.go`
  - sidebar/file diff summary: `internal/tui/components/chat/sidebar.go`
- Dialogs:
  - help
  - quit
  - session picker
  - command palette
  - model picker
  - permission prompt
  - init dialog
  - file picker / attachments
  - theme picker
  - multi-argument prompt for custom commands
- Layout primitives: `internal/tui/layout/*`
- Status bar shows:
  - help hint
  - current model
  - token/context usage
  - session cost
  - LSP project diagnostics

## Keybindings Worth Remembering
- Global (`internal/tui/tui.go`):
  - `ctrl+l` logs
  - `ctrl+c` quit
  - `ctrl+h` or `ctrl+_` help
  - `ctrl+s` switch session
  - `ctrl+k` command palette
  - `ctrl+f` file picker
  - `ctrl+o` model selection
  - `ctrl+t` theme selection
- Chat page (`internal/tui/page/chat.go`):
  - `@` file/folder completion
  - `ctrl+n` new session
  - `esc` cancel active agent run for current session

## Built-In And Custom Commands
- Built-ins registered in `internal/tui/tui.go`:
  - `Initialize Project`: asks the agent to create/update `MaebrCode.md`
  - `Compact Session`: triggers summarization
- Custom commands live in:
  - `$XDG_CONFIG_HOME/maebrcode/commands`
  - `$HOME/.maebrcode/commands`
  - `<repo>/.maebrcode/commands`
- Project command files are stored under `.maebrcode/commands`.
- Named args use `$NAME` placeholders and are handled by `internal/tui/components/dialog/custom_commands.go`.

## Themes
- Theme registry: `internal/tui/theme/manager.go`
- Built-in themes:
  - `maebrcode`
  - `catppuccin`
  - `dracula`
  - `flexoki`
  - `gruvbox`
  - `monokai`
  - `onedark`
  - `tokyonight`
  - `tron`

## Files To Touch For Common Change Types
- CLI flags/startup: `cmd/root.go`
- Config/defaults/schema: `internal/config/config.go`, `cmd/schema/main.go`, `maebrcode-schema.json`
- New provider/model: `internal/llm/models/*`, `internal/llm/provider/*`
- Prompt changes: `internal/llm/prompt/*`
- Tool changes: `internal/llm/tools/*`, then `internal/llm/agent/tools.go`
- Agent loop/summarization/tool orchestration: `internal/llm/agent/agent.go`
- DB schema/query changes: `internal/db/migrations/*`, `internal/db/sql/*`, then regenerate sqlc outputs
- TUI behavior/layout/keys: `internal/tui/tui.go`, `internal/tui/page/*`, `internal/tui/components/*`
- LSP/debugging: `internal/app/lsp.go`, `internal/lsp/client.go`, `internal/lsp/watcher/watcher.go`, `internal/llm/tools/diagnostics.go`

## Generated Or Low-Signal Areas
- `internal/lsp/protocol/*`: generated protocol bindings
- `internal/db/*sql.go`, `internal/db/db.go`, `internal/db/models.go`, `internal/db/querier.go`: generated by sqlc
- `maebrcode-schema.json`: generated from `cmd/schema`
- `go.sum`: dependency lockfile

## Tests Present
- `internal/llm/tools/ls_test.go`
- `internal/llm/prompt/prompt_test.go`
- `internal/tui/theme/theme_test.go`
- `internal/tui/components/dialog/custom_commands_test.go`
- Coverage is limited; many runtime-heavy areas are lightly or indirectly tested.
