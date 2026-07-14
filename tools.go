package main

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

func ExecuteTool(name string, args map[string]any) (string, error) {
	switch name {
	case "write_planner":
		content, ok := args["content"].(string)
		if !ok || content == "" {
			return "", fmt.Errorf("tool 'write_planner' requires a 'content' string argument")
		}

		if err := UpdatePlanner(content); err != nil {
			return "", fmt.Errorf("failed to write planner: %w", err)
		}

		return "✅ Success: PLANNER.md has been updated.", nil
	case "log_error":
		filePath, ok1 := args["file_path"].(string)
		mistake, ok2 := args["mistake"].(string)
		lastChanges, ok3 := args["last_changes"].(string)

		if !ok1 || !ok2 || !ok3 {
			return "", fmt.Errorf("tool 'log_error' requires 'file_path', 'mistake', and 'last_changes' strings")
		}

		if err := LogError(filePath, mistake, lastChanges); err != nil {
			return "", fmt.Errorf("failed to log error: %w", err)
		}

		return "✅ Success: Error logged. I will remember this.", nil
	case "read_errors":
		content, err := ReadErrors()
		if err != nil {
			return "", fmt.Errorf("failed to read errors: %w", err)
		}
		if content == "" {
			return "No known errors yet.", nil
		}
		return content, nil
	case "read_tasks":
		if err := LoadTasks(); err != nil {
			return "", fmt.Errorf("failed to load tasks: %w", err)
		}

		data, err := yaml.Marshal(State.TasksSystem)
		if err != nil {
			return "", fmt.Errorf("failed to format tasks: %w", err)
		}
		if len(State.TasksSystem.Tasks) == 0 {
			return "No tasks found. Use 'add_task' to create one.", nil
		}
		return string(data), nil

	case "add_task":
		id, ok1 := args["id"].(string)
		title, ok2 := args["title"].(string)
		desc, ok3 := args["description"].(string)

		if !ok1 || !ok2 || !ok3 {
			return "", fmt.Errorf("tool 'add_task' requires 'id', 'title', and 'description'")
		}

		if err := AddTask(id, title, desc); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Task '%s' created.", id), nil

	case "add_subtask":
		taskID, ok1 := args["task_id"].(string)
		subID, ok2 := args["sub_id"].(string)
		instruction, ok3 := args["instruction"].(string)
		filePath, _ := args["file_path"].(string)

		if !ok1 || !ok2 || !ok3 {
			return "", fmt.Errorf("tool 'add_subtask' requires 'task_id', 'sub_id' and 'instruction'")
		}

		if err := AddSubTask(taskID, subID, instruction, filePath); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Subtask '%s' added to '%s'.", subID, taskID), nil

	case "update_task_status":
		taskID, ok1 := args["task_id"].(string)
		status, ok2 := args["status"].(string)
		subID, _ := args["sub_id"].(string)

		if !ok1 || !ok2 {
			return "", fmt.Errorf("tool 'update_task_status' requires 'task_id' and 'status'")
		}

		if err := UpdateTaskStatus(taskID, subID, status); err != nil {
			return "", err
		}
		msg := fmt.Sprintf("✅ Status updated for %s %s -> %s", taskID, subID, status)
		if status == "DONE" && subID == "" { // whole task finished: checkpoint the working tree
			title := ""
			for _, t := range State.TasksSystem.Tasks {
				if t.ID == taskID {
					title = t.Title
				}
			}
			if note := gitCheckpoint(taskID, title); note != "" {
				msg += "\n" + note
			}
		}
		return msg, nil

	case "set_active_task":
		taskID, ok := args["task_id"].(string)
		if !ok {
			return "", fmt.Errorf("tool 'set_active_task' requires 'task_id'")
		}

		if err := SetCurrentTask(taskID); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Focus switched to task '%s'.", taskID), nil
	case "get_active_task":
		if err := LoadTasks(); err != nil {
			return "", fmt.Errorf("failed to load tasks: %w", err)
		}

		if len(State.TasksSystem.Tasks) == 0 {
			return "No tasks found. Use 'add_task' to create one.", nil
		}

		for _, task := range State.TasksSystem.Tasks {
			if task.ID == State.TasksSystem.CurrentTaskID {
				data, err := yaml.Marshal(task)
				if err != nil {
					return "", fmt.Errorf("failed to format tasks: %w", err)
				}
				return string(data), nil
			}
		}

		return "", fmt.Errorf("active task ID '%s' not found in task list", State.TasksSystem.CurrentTaskID)

	case "read_file":
		path, ok := args["path"].(string)
		if !ok {
			return "", fmt.Errorf("tool 'read_file' requires 'path'")
		}
		content, err := readFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		return content, nil

	case "write_file":
		path, ok1 := args["path"].(string)
		content, ok2 := args["content"].(string)
		if !ok1 || !ok2 {
			return "", fmt.Errorf("tool 'write_file' requires 'path' and 'content'")
		}
		if err := writeFile(path, []byte(content)); err != nil {
			return "", fmt.Errorf("failed to write file: %w", err)
		}
		msg := fmt.Sprintf("✅ File %s written successfully.", path)
		if invErr := UpdateInventoryFromFile(path); invErr != nil {
			msg += "\n⚠️ Inventory not updated: " + invErr.Error()
		}
		return msg, nil

	case "patch_file":
		path, ok1 := args["path"].(string)
		find, ok2 := args["find_snippet"].(string)
		replace, ok3 := args["replace_snippet"].(string)

		if !ok1 || !ok2 || !ok3 {
			return "", fmt.Errorf("tool 'patch_file' requires 'path', 'find_snippet', and 'replace_snippet'")
		}

		if err := PatchFile(path, find, replace); err != nil {
			return "", err
		}
		msg := fmt.Sprintf("✅ File %s patched successfully.", path)
		if invErr := UpdateInventoryFromFile(path); invErr != nil {
			msg += "\n⚠️ Inventory not updated: " + invErr.Error()
		}
		return msg, nil

	case "read_stack":
		if err := LoadStack(); err != nil {
			return "", fmt.Errorf("failed to load stack: %w", err)
		}
		data, _ := yaml.Marshal(State.Stack)
		return string(data), nil

	case "write_stack":
		// ponytail: tool schema passes one YAML string; unmarshal it into the generic update map
		if raw, ok := args["yaml"].(string); ok {
			updates := map[string]any{}
			if err := yaml.Unmarshal([]byte(raw), &updates); err != nil {
				return "", fmt.Errorf("invalid YAML for 'write_stack': %w", err)
			}
			args = updates
		}
		if err := UpdateStackGeneric(args); err != nil {
			return "", fmt.Errorf("failed to update stack: %w", err)
		}
		return "✅ Stack configuration updated.", nil

	case "add_dependency":
		name, ok1 := args["name"].(string)
		purpose, ok2 := args["purpose"].(string)

		if !ok1 || !ok2 {
			return "", fmt.Errorf("tool 'add_dependency' requires 'name' and 'purpose'")
		}

		if err := AddDependency(name, purpose); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Dependency '%s' added to stack.", name), nil

	case "read_inventory":
		if err := LoadInventory(); err != nil {
			return "", fmt.Errorf("failed to load inventory: %w", err)
		}
		data, _ := yaml.Marshal(State.Inventory)
		return string(data), nil

	case "read_flow":
		if err := LoadFlow(); err != nil {
			return "", fmt.Errorf("failed to load flow: %w", err)
		}
		data, _ := yaml.Marshal(State.Flow)
		return string(data), nil

	case "add_phase":
		id, ok1 := args["id"].(string)
		name, ok2 := args["name"].(string)
		desc, ok3 := args["description"].(string)

		if !ok1 || !ok2 || !ok3 {
			return "", fmt.Errorf("tool 'add_phase' requires id, name, description")
		}

		if err := AddPhase(id, name, desc); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Phase '%s' added.", name), nil

	case "set_active_phase":
		id, ok := args["id"].(string)
		if !ok {
			return "", fmt.Errorf("tool 'set_active_phase' requires id")
		}

		if err := SetActivePhase(id); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Current phase set to '%s'.", id), nil

	case "get_active_flow":
		if err := LoadFlow(); err != nil {
			return "", fmt.Errorf("failed to load flow: %w", err)
		}

		if len(State.Flow.Phases) == 0 {
			return "No phases found. Use 'add_phase' to create one.", nil
		}

		for _, phase := range State.Flow.Phases {
			if phase.ID == State.Flow.CurrentPhaseID {
				data, err := yaml.Marshal(phase)
				if err != nil {
					return "", fmt.Errorf("failed to format active phase: %w", err)
				}
				return string(data), nil
			}
		}

		return "", fmt.Errorf("active phase ID '%s' not found in flow list", State.Flow.CurrentPhaseID)

	case "run_command":
		cmdStr, ok := args["command"].(string)
		if !ok || cmdStr == "" {
			return "", fmt.Errorf("tool 'run_command' requires a 'command' string")
		}

		if !confirmCommand(cmdStr) {
			return "❌ Command denied by user. Ask before trying something similar.", nil
		}

		output, err := RunShellCommand(cmdStr)
		if err != nil {
			return fmt.Sprintf("❌ Error:\n%v", err), nil
		}

		if output == "" {
			return "✅ Command executed (no output).", nil
		}
		return fmt.Sprintf("✅ Command executed, Output:\n%s", output), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
