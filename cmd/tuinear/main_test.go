package main

import (
	"os"
	"testing"
)

func TestRunVersion(t *testing.T) {
	if code := run([]string{"--version"}); code != 0 {
		t.Fatalf("run returned %d", code)
	}
}

func TestRunRequiresToken(t *testing.T) {
	oldAPIKey, hadAPIKey := os.LookupEnv("LINEAR_API_KEY")
	oldClientID, hadClientID := os.LookupEnv("TUINEAR_OAUTH_CLIENT_ID")
	t.Cleanup(func() {
		if hadAPIKey {
			_ = os.Setenv("LINEAR_API_KEY", oldAPIKey)
		} else {
			_ = os.Unsetenv("LINEAR_API_KEY")
		}
		if hadClientID {
			_ = os.Setenv("TUINEAR_OAUTH_CLIENT_ID", oldClientID)
		} else {
			_ = os.Unsetenv("TUINEAR_OAUTH_CLIENT_ID")
		}
	})
	_ = os.Unsetenv("LINEAR_API_KEY")
	_ = os.Unsetenv("TUINEAR_OAUTH_CLIENT_ID")
	if code := run(nil); code != 2 {
		t.Fatalf("run returned %d, want 2", code)
	}
}
