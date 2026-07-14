# agent-go

An autonomous AI coding agent in Go, powered by the Gemini API. It plans a
project with you, breaks it into phases and tasks, then implements them by
calling tools (write/patch files, run commands) — with all of its state kept
in plain files you can read and edit.

## How to run

1. Get a Gemini API key: https://aistudio.google.com/apikey
2. Create a `.env` file in the directory you run the agent from:

   ```env
   GEMINI_API_KEY=your-key-here
   GEMINI_MODEL=gemini-3.5-flash
   RATE_LIMIT_RPM=10
   RATE_LIMIT_RPD=250
   ```

   Real environment variables override `.env` values.

3. Run it:

   ```sh
   go run .
   ```

## What happens

On first run the agent walks a pipeline, skipping any stage that is already done:

1. **Blueprint** — it interviews you about your idea (one question at a time,
   answer at the `you>` prompt) and writes a full PRD to `.agent-go/PLANNER.md`.
2. **Roadmap** — it derives sequential phases into `.agent-go/FLOW.yaml`.
3. **Tasks** — it breaks the active phase into tasks/subtasks in `.agent-go/TASKS.yaml`.
4. **REPL** — at the `>` prompt, tell it what to do (e.g. "work on the active
   task"). It writes code, verifies with `go build`/`go test`, and tracks progress.

Responses stream live, and transient API errors (429/5xx) are retried with
exponential backoff. Shell commands the agent wants to run are shown to you
for approval first (`go ...` commands are auto-approved). Type `exit` to quit.

Each completed task is committed as a git checkpoint (`agent: T-1 <title>`),
so any step is revertible — note this means you should run the agent in the
target project's own directory, where committing everything is what you want.
When every task of a phase is DONE, the agent advances to the next phase and
generates its task list automatically, until the roadmap is finished.

## State files (`.agent-go/`)

| File | Purpose |
|---|---|
| `PLANNER.md` | Project blueprint (PRD) |
| `STACK.yaml` | Tech stack rules and allowed dependencies |
| `FLOW.yaml` | Phase roadmap with one ACTIVE phase |
| `TASKS.yaml` | Tasks/subtasks for the active phase |
| `INVENTORY.yaml` | Symbol map of written Go code, auto-generated via `go/parser` |
| `ERRORS.md` | Past mistakes, re-injected into the prompt so they aren't repeated |
| `RATELIMIT.yaml` | Request timestamps for the persistent rate limiter |

Delete a file to make the agent regenerate that stage from scratch.

## Rate limiting

Every model call goes through a local limiter. Hitting the per-minute limit
waits; hitting the daily limit stops with an error telling you when it resets.
The request log lives on disk, so restarting the agent doesn't reset quotas.
