package main

import (
	"fmt"
	"strings"
)

func BuildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString("You are an autonomous AI Developer. You build software by executing tools.\n\n")
	sb.WriteString("--- WORK LOOP ---\n")
	sb.WriteString("1. Look at the CURRENT FOCUS task below; use set_active_task if none is selected.\n")
	sb.WriteString("2. Implement its subtasks one by one: write_file for new files, patch_file for edits (read_file first).\n")
	sb.WriteString("3. Mark each finished subtask/task DONE with update_task_status.\n")
	sb.WriteString("4. Verify with run_command (go build ./..., go test ./...) before declaring a task done.\n")
	sb.WriteString("5. If you made a mistake and fixed it, record it with log_error so it is never repeated.\n")
	sb.WriteString("6. When every task of the phase is DONE, report it and stop.\n\n")
	sb.WriteString("--- RULES ---\n")
	sb.WriteString("- Check read_inventory before creating a symbol; reuse what already exists. The inventory refreshes itself after every file write, and a parse warning there means you wrote invalid Go.\n")
	sb.WriteString("- Only use dependencies allowed by the stack; register new ones with add_dependency.\n")
	sb.WriteString("- Stay inside the scope of the active task; do not touch other phases.\n\n")

	if err := LoadStack(); err == nil {
		sb.WriteString("--- TECH STACK RULES ---\n")
		sb.WriteString(fmt.Sprintf("Language: %s\n", State.Stack.Language))
		sb.WriteString(fmt.Sprintf("Framework: %s\n", State.Stack.Framework))
		sb.WriteString(fmt.Sprintf("DB: %s\n", State.Stack.Database.Name))
		sb.WriteString("Allowed Dependencies:\n")
		for _, dep := range State.Stack.Dependencies {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", dep.Name, dep.Purpose))
		}
		sb.WriteString("\n")
	}

	if err := LoadFlow(); err == nil {
		sb.WriteString("--- PROJECT ROADMAP ---\n")
		sb.WriteString(fmt.Sprintf("Current Phase: %s\n", State.Flow.CurrentPhaseID))
		for _, p := range State.Flow.Phases {
			status := p.Status
			if p.ID == State.Flow.CurrentPhaseID {
				status = "⏩ ACTIVE"
			}
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", status, p.Name, p.Description))
		}
		sb.WriteString("\n")
	}

	if err := LoadTasks(); err == nil {
		sb.WriteString("--- CURRENT TASKS ---\n")
		foundActive := false
		for _, t := range State.TasksSystem.Tasks {
			if t.ID == State.TasksSystem.CurrentTaskID {
				sb.WriteString(fmt.Sprintf("👉 CURRENT FOCUS (%s): %s\n", t.ID, t.Title))
				sb.WriteString(fmt.Sprintf("   Description: %s\n", t.Description))
				sb.WriteString("   Subtasks:\n")
				for _, sub := range t.Subtasks {
					status := sub.Status
					if status == "" {
						status = "PENDING"
					}
					sb.WriteString(fmt.Sprintf("   - [%s] %s (File: %s)\n", status, sub.Instruction, sub.FilePath))
				}
				foundActive = true
			}
		}
		if !foundActive {
			sb.WriteString("No active task selected. Please use 'read_tasks' and 'set_active_task'.\n")
		}
		sb.WriteString("\n")
	}

	errors, _ := ReadErrors()
	if len(errors) > 4000 { // ponytail: byte cap on append-only log; summarize old entries if this ever matters
		errors = "(older errors truncated)\n" + errors[len(errors)-4000:]
	}
	if errors != "" {
		sb.WriteString("--- PAST MISTAKES (DO NOT REPEAT) ---\n")
		sb.WriteString(errors)
		sb.WriteString("\n")
	}

	plan, _ := ReadPlanner()
	if plan != "" {
		sb.WriteString("--- YOUR NOTES (PLANNER.md) ---\n")
		sb.WriteString(plan)
		sb.WriteString("\n")
	}

	return sb.String()
}
