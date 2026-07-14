package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-yaml"
)

// Free-tier defaults for gemini flash; overridden from .env in main.
var (
	rateLimitRPM = 10
	rateLimitRPD = 250
)

type requestLog struct {
	Requests []time.Time `yaml:"requests"`
}

// WaitForRateLimit blocks until a model call is allowed, then records it.
// The log is persisted to disk so restarting the agent never resets the daily quota.
func WaitForRateLimit() error {
	path := filepath.Join(aiPath, "RATELIMIT.yaml")

	var rl requestLog
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &rl)
	}

	// Keep only the last 24h (entries are in chronological order).
	now := time.Now()
	kept := rl.Requests[:0]
	for _, t := range rl.Requests {
		if now.Sub(t) < 24*time.Hour {
			kept = append(kept, t)
		}
	}
	rl.Requests = kept

	if len(rl.Requests) >= rateLimitRPD {
		reset := rl.Requests[0].Add(24 * time.Hour)
		return fmt.Errorf("daily limit of %d requests reached; resets at %s", rateLimitRPD, reset.Format("15:04"))
	}

	for {
		now = time.Now()
		inMinute := 0
		var oldest time.Time
		for _, t := range rl.Requests {
			if now.Sub(t) < time.Minute {
				if inMinute == 0 {
					oldest = t
				}
				inMinute++
			}
		}
		if inMinute < rateLimitRPM {
			break
		}
		wait := time.Until(oldest.Add(time.Minute)) + 100*time.Millisecond
		fmt.Printf("⏳ Rate limit (%d/min): waiting %s...\n", rateLimitRPM, wait.Round(time.Second))
		time.Sleep(wait)
	}

	rl.Requests = append(rl.Requests, time.Now())
	data, err := yaml.Marshal(rl)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
