package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

// Overridden by GEMINI_MODEL in .env.
var modelName = "gemini-3.5-flash"

func NewLLMClient(ctx context.Context) (*genai.Client, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}
	return genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  key,
		Backend: genai.BackendGeminiAPI,
	})
}

// RunAgent runs one user request to completion: model -> tool calls -> model ... -> final text.
func RunAgent(ctx context.Context, client *genai.Client, userPrompt string) error {
	return runToolLoop(ctx, client, BuildSystemPrompt(), agentTools(), userPrompt)
}

// runToolLoop drives one request to completion against a given system prompt and toolset.
func runToolLoop(ctx context.Context, client *genai.Client, system string, tools []*genai.Tool, userMsg string) error {
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(system, genai.RoleUser),
		Tools:             tools,
	}

	chat, err := client.Chats.Create(ctx, modelName, config, nil)
	if err != nil {
		return fmt.Errorf("failed to create chat: %w", err)
	}

	parts := []genai.Part{{Text: userMsg}}
	for {
		_, calls, err := sendWithRetry(ctx, chat, parts)
		if err != nil {
			return fmt.Errorf("model call failed: %w", err)
		}
		if len(calls) == 0 {
			return nil // final text was already streamed to the terminal
		}
		parts = execCalls(calls)
	}
}

// sendWithRetry sends one message with rate limiting, live-streamed output,
// and exponential backoff on transient API errors (429/5xx).
func sendWithRetry(ctx context.Context, chat *genai.Chat, parts []genai.Part) (string, []*genai.FunctionCall, error) {
	for attempt := 0; ; attempt++ {
		if err := WaitForRateLimit(); err != nil {
			return "", nil, err
		}
		text, calls, err := streamOnce(ctx, chat, parts)
		if err == nil || attempt >= 3 || !retryableAPIError(err) {
			return text, calls, err
		}
		wait := time.Duration(2<<attempt) * time.Second // 2s, 4s, 8s, 16s
		fmt.Printf("🔁 Transient API error (%v); retrying in %s...\n", err, wait)
		time.Sleep(wait)
	}
}

// streamOnce streams one model turn, printing text as it arrives and
// collecting any function calls.
func streamOnce(ctx context.Context, chat *genai.Chat, parts []genai.Part) (string, []*genai.FunctionCall, error) {
	var text strings.Builder
	var calls []*genai.FunctionCall
	for chunk, err := range chat.SendMessageStream(ctx, parts...) {
		if err != nil {
			if text.Len() > 0 {
				fmt.Println()
			}
			return text.String(), calls, err
		}
		if t := chunk.Text(); t != "" {
			fmt.Print(t)
			text.WriteString(t)
		}
		calls = append(calls, chunk.FunctionCalls()...)
	}
	if text.Len() > 0 {
		fmt.Println()
	}
	return text.String(), calls, nil
}

func retryableAPIError(err error) bool {
	if ae, ok := errors.AsType[genai.APIError](err); ok {
		return ae.Code == 429 || ae.Code >= 500
	}
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "503") || strings.Contains(s, "RESOURCE_EXHAUSTED") || strings.Contains(s, "UNAVAILABLE")
}

// execCalls executes tool calls and packages the results as function responses.
func execCalls(calls []*genai.FunctionCall) []genai.Part {
	var parts []genai.Part
	for _, call := range calls {
		fmt.Printf("🔧 %s %v\n", call.Name, call.Args)
		result, err := ExecuteTool(call.Name, call.Args)
		if err != nil {
			result = "❌ " + err.Error() // feed the error back so the model can self-correct
		}
		parts = append(parts, genai.Part{FunctionResponse: &genai.FunctionResponse{
			Name:     call.Name,
			Response: map[string]any{"result": result},
		}})
	}
	return parts
}

// pick returns a toolset restricted to the named declarations.
func pick(names ...string) []*genai.Tool {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []*genai.FunctionDeclaration
	for _, fd := range agentTools()[0].FunctionDeclarations {
		if want[fd.Name] {
			out = append(out, fd)
		}
	}
	return []*genai.Tool{{FunctionDeclarations: out}}
}

func str(desc string) *genai.Schema {
	return &genai.Schema{Type: genai.TypeString, Description: desc}
}

func decl(name, desc string, props map[string]*genai.Schema, required ...string) *genai.FunctionDeclaration {
	fd := &genai.FunctionDeclaration{Name: name, Description: desc}
	if len(props) > 0 {
		fd.Parameters = &genai.Schema{Type: genai.TypeObject, Properties: props, Required: required}
	}
	return fd
}

func agentTools() []*genai.Tool {
	return []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{
		// Planner
		decl("write_planner", "Overwrite PLANNER.md with the full project blueprint (markdown).",
			map[string]*genai.Schema{"content": str("Full markdown blueprint")}, "content"),

		// Errors
		decl("log_error", "Record a mistake so it is never repeated.",
			map[string]*genai.Schema{
				"file_path":    str("File where the mistake happened"),
				"mistake":      str("What went wrong"),
				"last_changes": str("What was changed"),
			}, "file_path", "mistake", "last_changes"),
		decl("read_errors", "Read all previously logged mistakes.", nil),

		// Tasks
		decl("read_tasks", "Read the full task list for the current phase.", nil),
		decl("add_task", "Add a new task to the current phase.",
			map[string]*genai.Schema{
				"id":          str("Task ID, format T-<number>"),
				"title":       str("Short title"),
				"description": str("What this task implements"),
			}, "id", "title", "description"),
		decl("add_subtask", "Add an atomic subtask to an existing task.",
			map[string]*genai.Schema{
				"task_id":     str("Parent task ID"),
				"sub_id":      str("Subtask ID, format S-<number>"),
				"instruction": str("Atomic executable instruction"),
				"file_path":   str("Target file path, empty for build/test steps"),
			}, "task_id", "sub_id", "instruction"),
		decl("update_task_status", "Set status of a task or subtask to PENDING, IN_PROGRESS or DONE.",
			map[string]*genai.Schema{
				"task_id": str("Task ID"),
				"sub_id":  str("Subtask ID, omit to update the task itself"),
				"status":  str("PENDING, IN_PROGRESS or DONE"),
			}, "task_id", "status"),
		decl("set_active_task", "Switch focus to a task.",
			map[string]*genai.Schema{"task_id": str("Task ID")}, "task_id"),
		decl("get_active_task", "Get the currently focused task with its subtasks.", nil),

		// Files
		decl("read_file", "Read a file from the project.",
			map[string]*genai.Schema{"path": str("Relative file path")}, "path"),
		decl("write_file", "Create or overwrite a file.",
			map[string]*genai.Schema{
				"path":    str("Relative file path"),
				"content": str("Full file content"),
			}, "path", "content"),
		decl("patch_file", "Replace an exact snippet in a file. Read the file first.",
			map[string]*genai.Schema{
				"path":            str("Relative file path"),
				"find_snippet":    str("Exact text to find"),
				"replace_snippet": str("Replacement text"),
			}, "path", "find_snippet", "replace_snippet"),

		// Stack
		decl("read_stack", "Read the tech stack rules (STACK.yaml).", nil),
		decl("write_stack", "Update the tech stack. Pass a YAML document matching the STACK.yaml schema.",
			map[string]*genai.Schema{"yaml": str("YAML document with fields to update")}, "yaml"),
		decl("add_dependency", "Register an allowed dependency in the stack.",
			map[string]*genai.Schema{
				"name":    str("Package name/module path"),
				"purpose": str("Why it is needed"),
			}, "name", "purpose"),

		// Inventory (updated automatically from source via go/parser after every file write)
		decl("read_inventory", "Read the code inventory of existing symbols.", nil),

		// Flow
		decl("read_flow", "Read the full project roadmap (all phases).", nil),
		decl("get_active_flow", "Get only the currently active phase.", nil),
		decl("add_phase", "Add a phase to the roadmap.",
			map[string]*genai.Schema{
				"id":          str("Phase ID, format P-<number>"),
				"name":        str("Short phase name"),
				"description": str("What is built in this phase and with which stack technologies"),
			}, "id", "name", "description"),
		decl("set_active_phase", "Mark a phase ACTIVE (the previous active phase becomes DONE).",
			map[string]*genai.Schema{"id": str("Phase ID")}, "id"),

		// Shell
		decl("run_command", "Run a shell command (build, test, git...) and return its output.",
			map[string]*genai.Schema{"command": str("The command to execute")}, "command"),
	}}}
}
