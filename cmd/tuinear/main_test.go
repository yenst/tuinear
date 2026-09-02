package main

import (
	"os"
	"strings"
	"testing"

	"github.com/jihmy/tuinear/internal/auth"
)

func TestRunVersion(t *testing.T) {
	if code := run([]string{"--version"}); code != 0 {
		t.Fatalf("run returned %d", code)
	}
}

func TestRunHelp(t *testing.T) {
	if code := run([]string{"--help"}); code != 0 {
		t.Fatalf("run returned %d", code)
	}
}

func TestResolveProfile(t *testing.T) {
	profiles := []auth.Profile{
		{ID: "profile-work", WorkspaceName: "Acme", WorkspaceKey: "acme", UserName: "Jamie", UserEmail: "jamie@work.test"},
		{ID: "profile-personal", WorkspaceName: "Personal", WorkspaceKey: "personal", UserName: "Jamie", UserEmail: "jamie@home.test"},
	}
	for _, selector := range []string{"profile-work", "Acme", "acme", "jamie@work.test"} {
		profile, err := resolveProfile(profiles, selector)
		if err != nil || profile.ID != "profile-work" {
			t.Fatalf("resolveProfile(%q) = %#v, %v", selector, profile, err)
		}
	}
	if _, err := resolveProfile(profiles, "Jamie"); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("ambiguous error = %v", err)
	}
	if _, err := resolveProfile(profiles, "missing"); err == nil || !strings.Contains(err.Error(), "no saved") {
		t.Fatalf("missing error = %v", err)
	}
}

func TestPrintProfilesMarksActive(t *testing.T) {
	profiles := []auth.Profile{
		{ID: "work", WorkspaceName: "Acme", WorkspaceKey: "acme", UserName: "Jamie", UserEmail: "jamie@work.test"},
		{ID: "personal", WorkspaceName: "Personal", WorkspaceKey: "personal", UserName: "Jamie"},
	}
	var output strings.Builder
	printProfiles(&output, profiles, "personal")
	if !strings.Contains(output.String(), "  work") || !strings.Contains(output.String(), "* personal") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestConfiguredOAuthClientID(t *testing.T) {
	oldClientID, hadClientID := os.LookupEnv("TUINEAR_OAUTH_CLIENT_ID")
	t.Cleanup(func() {
		if hadClientID {
			_ = os.Setenv("TUINEAR_OAUTH_CLIENT_ID", oldClientID)
		} else {
			_ = os.Unsetenv("TUINEAR_OAUTH_CLIENT_ID")
		}
	})
	_ = os.Unsetenv("TUINEAR_OAUTH_CLIENT_ID")
	if got := configuredOAuthClientID(); got != "3c2a2e12d13e32eaaa0a3d69de27aa61" {
		t.Fatalf("default client ID = %q", got)
	}
	_ = os.Setenv("TUINEAR_OAUTH_CLIENT_ID", "override-client")
	if got := configuredOAuthClientID(); got != "override-client" {
		t.Fatalf("overridden client ID = %q", got)
	}
}
