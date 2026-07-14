package main

import (
	"os"
	"testing"
)

func TestRateLimitDailyQuota(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(aiPath, 0755); err != nil {
		t.Fatal(err)
	}

	oldRPM, oldRPD := rateLimitRPM, rateLimitRPD
	rateLimitRPM, rateLimitRPD = 1000, 2 // high RPM so the test never sleeps
	defer func() { rateLimitRPM, rateLimitRPD = oldRPM, oldRPD }()

	for i := range 2 {
		if err := WaitForRateLimit(); err != nil {
			t.Fatalf("request %d unexpectedly limited: %v", i+1, err)
		}
	}
	if err := WaitForRateLimit(); err == nil {
		t.Fatal("third request should have hit the daily limit of 2")
	}
}
