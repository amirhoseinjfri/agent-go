package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/genai"
)

var stdin = bufio.NewScanner(os.Stdin)

func main() {
	stdin.Buffer(make([]byte, 1024*1024), 1024*1024)

	loadDotEnv()
	modelName = envOr("GEMINI_MODEL", modelName)
	rateLimitRPM = envIntOr("RATE_LIMIT_RPM", rateLimitRPM)
	rateLimitRPD = envIntOr("RATE_LIMIT_RPD", rateLimitRPD)

	if err := InitializeAgent(); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := NewLLMClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if err := bootstrap(ctx, client); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nReady. Tell the agent what to do (or 'exit'):")
	for {
		fmt.Print("> ")
		if !stdin.Scan() {
			break
		}
		input := strings.TrimSpace(stdin.Text())
		if input == "" {
			continue
		}
		if input == "exit" {
			break
		}
		if err := RunAgent(ctx, client, input); err != nil {
			log.Println("agent error:", err)
			continue
		}
		if err := maybeAdvancePhase(ctx, client); err != nil {
			log.Println("phase advance error:", err)
		}
	}
	if err := stdin.Err(); err != nil {
		log.Fatal(err)
	}
}

// bootstrap runs whichever pipeline stages are still missing: blueprint -> roadmap -> tasks.
func bootstrap(ctx context.Context, client *genai.Client) error {
	plan, _ := ReadPlanner()
	if strings.TrimSpace(plan) == "" {
		fmt.Print("No blueprint found. Describe your project idea:\n> ")
		if !stdin.Scan() {
			return fmt.Errorf("no input")
		}
		fmt.Println("📐 Planning...")
		if err := MakePlan(ctx, client, stdin.Text()); err != nil {
			return err
		}
	}

	if err := LoadFlow(); err != nil {
		return err
	}
	if len(State.Flow.Phases) == 0 {
		fmt.Println("🗺  Generating roadmap...")
		if err := GenerateFlow(ctx, client); err != nil {
			return err
		}
	}

	if err := LoadTasks(); err != nil {
		return err
	}
	if len(State.TasksSystem.Tasks) == 0 {
		fmt.Println("📋 Generating tasks for the active phase...")
		if err := GenerateTasks(ctx, client); err != nil {
			return err
		}
	}
	return nil
}

// confirmCommand gates shell execution behind user approval.
func confirmCommand(cmd string) bool {
	if strings.HasPrefix(cmd, "go ") { // ponytail: allowlist go build/test/vet; widen if prompts get annoying
		return true
	}
	fmt.Printf("⚠️  Agent wants to run: %s\nAllow? [y/N] ", cmd)
	if !stdin.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(stdin.Text()))
	return ans == "y" || ans == "yes"
}
