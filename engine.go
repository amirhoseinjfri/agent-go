package main

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/goccy/go-yaml"
	"google.golang.org/genai"
)

//go:embed prompts/planner.md
var plannerPrompt string

//go:embed prompts/flow.md
var flowPromptTmpl string

//go:embed prompts/task.md
var taskPromptTmpl string

// MakePlan runs the interactive planner interview: the model asks one question
// at a time and finishes by saving the blueprint via the write_planner tool.
func MakePlan(ctx context.Context, client *genai.Client, idea string) error {
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(plannerPrompt, genai.RoleUser),
		Tools:             pick("write_planner"),
	}
	chat, err := client.Chats.Create(ctx, modelName, config, nil)
	if err != nil {
		return fmt.Errorf("failed to create chat: %w", err)
	}

	parts := []genai.Part{{Text: idea}}
	wrote := false
	for {
		_, calls, err := sendWithRetry(ctx, chat, parts)
		if err != nil {
			return fmt.Errorf("model call failed: %w", err)
		}

		if len(calls) > 0 {
			for _, c := range calls {
				wrote = wrote || c.Name == "write_planner"
			}
			parts = execCalls(calls)
			continue
		}

		if wrote {
			return nil
		}
		fmt.Print("you> ")
		if !stdin.Scan() {
			return fmt.Errorf("input closed before the blueprint was finished")
		}
		parts = []genai.Part{{Text: stdin.Text()}}
	}
}

// GenerateFlow builds the project roadmap via add_phase/set_active_phase tool calls.
func GenerateFlow(ctx context.Context, client *genai.Client) error {
	plan, err := ReadPlanner()
	if err != nil {
		return err
	}
	if err := LoadStack(); err != nil {
		return err
	}
	if err := LoadFlow(); err != nil {
		return err
	}
	stackY, _ := yaml.Marshal(State.Stack)
	flowY, _ := yaml.Marshal(State.Flow)

	system := fmt.Sprintf(flowPromptTmpl, plan, string(stackY), string(flowY))
	return runToolLoop(ctx, client, system, pick("add_phase", "set_active_phase"),
		"Generate the project roadmap now using the tools.")
}

// GenerateTasks builds the ACTIVE phase's task list via add_task/add_subtask/set_active_task tool calls.
func GenerateTasks(ctx context.Context, client *genai.Client) error {
	plan, err := ReadPlanner()
	if err != nil {
		return err
	}
	if err := LoadStack(); err != nil {
		return err
	}
	if err := LoadInventory(); err != nil {
		return err
	}
	if err := LoadFlow(); err != nil {
		return err
	}
	stackY, _ := yaml.Marshal(State.Stack)
	invY, _ := yaml.Marshal(State.Inventory)
	flowY, _ := yaml.Marshal(State.Flow)

	system := fmt.Sprintf(taskPromptTmpl, plan, string(stackY), string(invY), string(flowY))
	return runToolLoop(ctx, client, system, pick("add_task", "add_subtask", "set_active_task"),
		"Generate the task list for the ACTIVE phase now using the tools.")
}

// maybeAdvancePhase moves to the next phase once every task of the active
// phase is DONE: marks the phase done, activates the next one, and generates
// its task list. Prints a finish message when there is no next phase.
func maybeAdvancePhase(ctx context.Context, client *genai.Client) error {
	if err := LoadTasks(); err != nil {
		return err
	}
	if len(State.TasksSystem.Tasks) == 0 {
		return nil
	}
	for _, t := range State.TasksSystem.Tasks {
		if t.Status != "DONE" {
			return nil
		}
	}

	if err := LoadFlow(); err != nil {
		return err
	}
	var next *Phase
	for i, p := range State.Flow.Phases {
		if p.ID == State.Flow.CurrentPhaseID && i+1 < len(State.Flow.Phases) {
			next = &State.Flow.Phases[i+1]
			break
		}
	}
	if next == nil {
		fmt.Println("🏁 All phases complete. Project finished!")
		return nil
	}

	fmt.Printf("🎉 Phase %s complete — advancing to %s (%s).\n", State.Flow.CurrentPhaseID, next.ID, next.Name)
	if err := SetActivePhase(next.ID); err != nil {
		return err
	}
	State.TasksSystem = TaskSystem{Tasks: []Task{}} // fresh task list; finished work is preserved in git checkpoints
	if err := SaveTasks(); err != nil {
		return err
	}
	fmt.Println("📋 Generating tasks for the new phase...")
	return GenerateTasks(ctx, client)
}
