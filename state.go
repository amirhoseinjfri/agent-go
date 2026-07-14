package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goccy/go-yaml"
)

var State AgentState

const aiPath = ".agent-go"

func InitializeAgent() error {
	if err := os.MkdirAll(aiPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	files := []string{
		"PLANNER.md",
		"STACK.yaml",
		"FLOW.yaml",
		"TASKS.yaml",
		"INVENTORY.yaml",
		"ERRORS.md",
	}

	for _, name := range files {
		path := filepath.Join(aiPath, name)

		if err := createFileIfNotExist(path); err != nil {
			if os.IsExist(err) {
				log.Printf("🔸 Skipped to create %s: %v\n", name, "already exist")
				continue
			} else {
				log.Printf("❌  Failed to create %s: %v\n", name, err)
				continue
			}
		}

		log.Printf("✅ File %s created successfully\n", name)
	}

	fmt.Println("\n🧬 agent-go initialized successfully!")
	return nil
}

// Planner
func LoadPlanner() error {
	path := filepath.Join(aiPath, "PLANNER.md")

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read PLANNER.md: %v", err)
	}

	State.Planner = string(content)

	return nil
}

func UpdatePlanner(content string) error {
	State.Planner = content

	path := filepath.Join(aiPath, "PLANNER.md")
	return os.WriteFile(path, []byte(content), 0644)
}

func ReadPlanner() (string, error) {
	path := filepath.Join(aiPath, "PLANNER.md")
	content, err := readFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Error
func ReadErrors() (string, error) {
	path := filepath.Join(aiPath, "ERRORS.md")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(content), nil
}

func LogError(filePath, mistake, lastChanges string) error {
	path := filepath.Join(aiPath, "ERRORS.md")
	entry := fmt.Sprintf("- **File Path:** %s\n  **Mistake:** %s\n  **Last Changes:** %s\n\n", filePath, mistake, lastChanges)
	return appendFile(path, []byte(entry))
}

// Task
func LoadTasks() error {
	path := filepath.Join(aiPath, "TASKS.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			State.TasksSystem = TaskSystem{Tasks: []Task{}}
			return nil
		}
		return err
	}

	return yaml.Unmarshal(content, &State.TasksSystem)
}

func SaveTasks() error {
	data, err := yaml.Marshal(&State.TasksSystem)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %v", err)
	}
	return writeFile(filepath.Join(aiPath, "TASKS.yaml"), data)
}

func SetCurrentTask(taskID string) error {
	exists := false
	for _, t := range State.TasksSystem.Tasks {
		if t.ID == taskID {
			exists = true
			break
		}
	}
	if !exists {
		return fmt.Errorf("task ID %s not found", taskID)
	}

	State.TasksSystem.CurrentTaskID = taskID
	return SaveTasks()
}

func AddTask(id, title, desc string) error {
	if err := LoadTasks(); err != nil {
		return err
	}

	currentPhase := GetCurrentPhaseID()
	newTask := Task{
		ID:          id,
		Title:       title,
		Description: desc,
		Status:      "PENDING",
		PhaseID:     currentPhase,
		Subtasks:    []SubTask{},
	}

	State.TasksSystem.Tasks = append(State.TasksSystem.Tasks, newTask)
	return SaveTasks()
}

func AddSubTask(taskID, subID, instruction, filePath string) error {
	if err := LoadTasks(); err != nil {
		return err
	}

	for i, t := range State.TasksSystem.Tasks {
		if t.ID == taskID {
			newSub := SubTask{
				ID:          subID,
				Instruction: instruction,
				Status:      "PENDING",
				FilePath:    filePath,
			}
			State.TasksSystem.Tasks[i].Subtasks = append(State.TasksSystem.Tasks[i].Subtasks, newSub)
			return SaveTasks()
		}
	}
	return fmt.Errorf("parent task ID '%s' not found", taskID)
}

func UpdateTaskStatus(taskID, subID, status string) error {
	if err := LoadTasks(); err != nil {
		return err
	}

	for i, t := range State.TasksSystem.Tasks {
		if t.ID == taskID {
			if subID != "" {
				for j, s := range t.Subtasks {
					if s.ID == subID {
						State.TasksSystem.Tasks[i].Subtasks[j].Status = status
						return SaveTasks()
					}
				}
				return fmt.Errorf("subtask ID '%s' not found", subID)
			}

			State.TasksSystem.Tasks[i].Status = status
			return SaveTasks()
		}
	}
	return fmt.Errorf("task ID '%s' not found", taskID)
}

// Stack
func LoadStack() error {
	path := filepath.Join(aiPath, "STACK.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			State.Stack = ProjectStack{}
			return nil
		}
		return err
	}

	return yaml.Unmarshal(data, &State.Stack)
}

func SaveStack() error {
	data, err := yaml.Marshal(&State.Stack)
	if err != nil {
		return fmt.Errorf("failed to marshal stack: %v", err)
	}
	return writeFile(filepath.Join(aiPath, "STACK.yaml"), data)
}

func UpdateStackGeneric(updates map[string]any) error {
	if err := LoadStack(); err != nil {
		return err
	}
	updateBytes, err := yaml.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to process updates: %v", err)
	}

	if err := yaml.Unmarshal(updateBytes, &State.Stack); err != nil {
		return fmt.Errorf("failed to apply updates to stack: %v", err)
	}

	return SaveStack()
}

func AddDependency(name, purpose string) error {
	if err := LoadStack(); err != nil {
		return err
	}

	for _, dep := range State.Stack.Dependencies {
		if dep.Name == name {
			return nil
		}
	}

	newDep := Dependency{
		Name:    name,
		Purpose: purpose,
	}

	State.Stack.Dependencies = append(State.Stack.Dependencies, newDep)
	return SaveStack()
}

// Inventory
func LoadInventory() error {
	path := filepath.Join(aiPath, "INVENTORY.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			State.Inventory = Inventory{
				Packages: make(map[string]PackageData),
			}
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, &State.Inventory)
}

func SaveInventory() error {
	data, err := yaml.Marshal(&State.Inventory)
	if err != nil {
		return fmt.Errorf("failed to marshal inventory: %v", err)
	}
	return writeFile(filepath.Join(aiPath, "INVENTORY.yaml"), data)
}

// Flow
func LoadFlow() error {
	path := filepath.Join(aiPath, "FLOW.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			State.Flow = ProjectFlow{Phases: []Phase{}}
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, &State.Flow)
}

func SaveFlow() error {
	data, err := yaml.Marshal(&State.Flow)
	if err != nil {
		return fmt.Errorf("failed to marshal flow: %v", err)
	}
	return writeFile(filepath.Join(aiPath, "FLOW.yaml"), data)
}

func AddPhase(id, name, desc string) error {
	if err := LoadFlow(); err != nil {
		return err
	}

	newPhase := Phase{
		ID:          id,
		Name:        name,
		Description: desc,
		Status:      "PENDING",
	}
	State.Flow.Phases = append(State.Flow.Phases, newPhase)
	return SaveFlow()
}

func SetActivePhase(phaseID string) error {
	if err := LoadFlow(); err != nil {
		return err
	}

	found := false
	for i, p := range State.Flow.Phases {
		if p.ID == phaseID {
			State.Flow.Phases[i].Status = "ACTIVE"
			found = true
		} else if p.Status == "ACTIVE" {
			State.Flow.Phases[i].Status = "DONE"
		}
	}

	if !found {
		return fmt.Errorf("phase ID '%s' not found", phaseID)
	}

	State.Flow.CurrentPhaseID = phaseID
	return SaveFlow()
}

// I/O
func createFileIfNotExist(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_EXCL|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	defer file.Close()
	return nil
}

func appendFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func PatchFile(path, findSnippet, replaceSnippet string) error {
	content, err := readFile(path)
	if err != nil {
		return fmt.Errorf("cannot patch non-existent file: %s", path)
	}

	if !strings.Contains(content, findSnippet) {
		return fmt.Errorf("PATCH FAILED: Exact snippet not found in %s. Read the file again.", path)
	}

	newContent := strings.Replace(content, findSnippet, replaceSnippet, 1)

	return os.WriteFile(path, []byte(newContent), 0644)
}

func GetCurrentPhaseID() string {
	if len(State.Flow.Phases) == 0 {
		_ = LoadFlow()
	}
	return State.Flow.CurrentPhaseID
}

// gitCheckpoint commits the working tree after a task completes so the
// agent's work is revertible step by step. Failures never block the agent.
func gitCheckpoint(taskID, title string) string {
	if _, err := os.Stat(".git"); err != nil {
		if _, err := RunShellCommand("git init"); err != nil {
			return "⚠️ git checkpoint skipped: " + err.Error()
		}
	}
	if _, err := RunShellCommand("git add -A"); err != nil {
		return "⚠️ git checkpoint skipped: " + err.Error()
	}
	title = strings.ReplaceAll(title, `"`, "'") // keep the shell-quoted message safe
	if _, err := RunShellCommand(fmt.Sprintf(`git commit -m "agent: %s %s"`, taskID, title)); err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			return ""
		}
		return "⚠️ git checkpoint failed: " + err.Error()
	}
	return "📌 Git checkpoint created."
}

func RunShellCommand(command string) (string, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("command failed: %s\nOutput: %s", err, result)
	}

	return result, nil
}
