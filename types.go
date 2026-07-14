package main

type AgentState struct {
	Planner     string
	Stack       ProjectStack
	TasksSystem TaskSystem
	Inventory   Inventory
	Flow        ProjectFlow
}

// Task
type TaskSystem struct {
	CurrentTaskID string `yaml:"current_task_id"`
	Tasks         []Task `yaml:"tasks"`
}

type Task struct {
	ID          string    `yaml:"id"`
	Title       string    `yaml:"title"`
	Description string    `yaml:"description"`
	Status      string    `yaml:"status"` // PENDING, IN_PROGRESS, DONE
	PhaseID     string    `yaml:"phase_id"`
	Subtasks    []SubTask `yaml:"subtasks"`
}

type SubTask struct {
	ID          string `yaml:"id"`
	Instruction string `yaml:"instruction"`
	FilePath    string `yaml:"file_path"`
	Status      string `yaml:"status"`
}

// Stack
type ProjectStack struct {
	Language     string `yaml:"language"`
	Framework    string `yaml:"framework"`
	Architecture string `yaml:"architecture"`

	Runtime string `yaml:"runtime"`
	OS      string `yaml:"os"`

	Database struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"`
	} `yaml:"database"`

	API struct {
		Style         string   `yaml:"style"`
		Protocols     []string `yaml:"protocols"`
		Serialization []string `yaml:"serialization"`
	} `yaml:"api"`

	Security struct {
		Auth       string `yaml:"auth"`
		Encryption string `yaml:"encryption"`
	} `yaml:"security"`

	Deployment struct {
		Method  string `yaml:"method"`
		Hosting string `yaml:"hosting"`
	} `yaml:"deployment"`

	Observability struct {
		Logging    string `yaml:"logging"`
		Monitoring string `yaml:"monitoring"`
		Tracing    string `yaml:"tracing"`
	} `yaml:"observability"`

	Dependencies []Dependency `yaml:"dependencies"`
}

type Dependency struct {
	Name    string `yaml:"name"`
	Purpose string `yaml:"purpose"`
}

// Inventory
type Inventory struct {
	Packages map[string]PackageData `yaml:"packages"`
}

type PackageData struct {
	Description string              `yaml:"description"`
	Items       map[string]CodeItem `yaml:"items"`
}

type CodeItem struct {
	Type           string `yaml:"type"`
	Signature      string `yaml:"signature"`
	Description    string `yaml:"description"`
	FilePath       string `yaml:"file_path"`
	CreatedInPhase string `yaml:"created_in_phase"`
}

// Flow
type ProjectFlow struct {
	CurrentPhaseID string  `yaml:"current_phase_id"`
	Phases         []Phase `yaml:"phases"`
}

type Phase struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Status      string `yaml:"status"` // PENDING, ACTIVE, DONE
}
