package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/auth"
	"github.com/jihmy/tuinear/internal/browser"
	cachepkg "github.com/jihmy/tuinear/internal/cache"
	"github.com/jihmy/tuinear/internal/linear"
	"github.com/jihmy/tuinear/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	// oauthClientID is public OAuth application configuration. It may still be
	// replaced at build time with -ldflags -X or at runtime through the env var.
	oauthClientID = "3c2a2e12d13e32eaaa0a3d69de27aa61"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("tuinear", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	demo := flags.Bool("demo", false, "use built-in sample tickets")
	snapshot := flags.Bool("snapshot", false, "print a demo screen and exit")
	login := flags.Bool("login", false, "sign in to another Linear workspace or user")
	logout := flags.Bool("logout", false, "revoke the selected Linear account")
	accounts := flags.Bool("accounts", false, "list saved workspace and user profiles")
	profileSelector := flags.String("profile", "", "use a profile by workspace, user, email, or ID")
	showVersion := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if boolCount(*login, *logout, *accounts, *demo, *snapshot) > 1 {
		fmt.Fprintln(os.Stderr, "Choose only one of --login, --logout, --accounts, --demo, or --snapshot.")
		return 2
	}
	if *profileSelector != "" && (*login || *accounts || *demo || *snapshot) {
		fmt.Fprintln(os.Stderr, "--profile can be used when opening Tuinear or together with --logout.")
		return 2
	}

	if *showVersion {
		fmt.Printf("tuinear %s (%s)\n", version, commit)
		return 0
	}
	if *snapshot {
		dashboard, err := (linear.DemoClient{}).FetchDashboard(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(ui.Snapshot(dashboard, 120, 32))
		return 0
	}

	var loader ui.DashboardLoader
	var cacheKey cachepkg.KeyFunc
	if *demo {
		loader = linear.DemoClient{}
	} else {
		apiKey := os.Getenv("LINEAR_API_KEY")
		if apiKey != "" {
			if *login || *logout || *accounts || *profileSelector != "" {
				fmt.Fprintln(os.Stderr, "Unset LINEAR_API_KEY before using OAuth account profiles.")
				return 2
			}
			loader = linear.NewClient(apiKey)
			cacheKey = func() (string, error) { return cachepkg.APIKeyCacheKey(apiKey), nil }
		} else {
			clientID := configuredOAuthClientID()
			if clientID == "" {
				fmt.Fprintln(os.Stderr, "TUINEAR_OAUTH_CLIENT_ID is not set. See README.md for one-time OAuth app setup, or run with --demo.")
				return 2
			}
			manager := auth.NewManager(clientID)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()

			profiles, err := manager.Profiles()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Load Linear accounts: %v\n", err)
				return 1
			}
			if *accounts {
				activeID, err := manager.ActiveProfileID()
				if errors.Is(err, auth.ErrProfileNotFound) {
					activeID = ""
				} else if err != nil {
					fmt.Fprintf(os.Stderr, "Load active Linear account: %v\n", err)
					return 1
				}
				printProfiles(os.Stdout, profiles, activeID)
				return 0
			}
			if *profileSelector != "" {
				profile, err := resolveProfile(profiles, *profileSelector)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return 2
				}
				if err := manager.SelectProfile(profile.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Select Linear account: %v\n", err)
					return 1
				}
			}
			if *logout {
				profile, _ := manager.ActiveProfile()
				if err := manager.Logout(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Logout failed: %v\n", err)
					return 1
				}
				if profile.Label() == "" {
					fmt.Println("No saved Linear account was selected.")
				} else {
					fmt.Printf("Logged out: %s.\n", profile.Label())
				}
				return 0
			}
			if *login {
				profile, err := loginWithBrowser(ctx, manager)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
					return 1
				}
				fmt.Printf("Connected: %s.\n", profile.Label())
				return 0
			}
			if _, err := manager.Token(ctx); errors.Is(err, auth.ErrNotLoggedIn) {
				fmt.Println("No saved Linear session; starting browser login.")
				profile, err := loginWithBrowser(ctx, manager)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
					return 1
				}
				fmt.Printf("Connected: %s.\n", profile.Label())
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "Load Linear session: %v\n", err)
				return 1
			}
			loader = newAccountLoader(manager)
			cacheKey = func() (string, error) {
				activeID, err := manager.ActiveProfileID()
				if err != nil {
					return "", err
				}
				return "oauth:" + activeID, nil
			}
		}
	}
	if cacheKey != nil {
		path, err := cachepkg.DefaultPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cache disabled: %v\n", err)
		} else {
			store, err := cachepkg.Open(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Cache disabled: %v\n", err)
			} else {
				defer store.Close()
				loader = cachepkg.NewLoader(store, loader, cacheKey)
			}
		}
	}

	program := tea.NewProgram(ui.NewWithBrowser(loader, browser.Open))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Tuinear failed: %v\n", err)
		return 1
	}
	return 0
}

func configuredOAuthClientID() string {
	if clientID := strings.TrimSpace(os.Getenv("TUINEAR_OAUTH_CLIENT_ID")); clientID != "" {
		return clientID
	}
	return oauthClientID
}

func loginWithBrowser(ctx context.Context, manager *auth.Manager) (auth.Profile, error) {
	return manager.Login(ctx, func(authorizeURL string) {
		fmt.Printf("Open this URL to connect Linear:\n%s\n", authorizeURL)
		if err := auth.OpenBrowser(authorizeURL); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open a browser automatically: %v\n", err)
		}
	})
}

func resolveProfile(profiles []auth.Profile, selector string) (auth.Profile, error) {
	selector = strings.TrimSpace(selector)
	for _, profile := range profiles {
		if profile.ID == selector {
			return profile, nil
		}
	}
	var matches []auth.Profile
	for _, profile := range profiles {
		values := []string{
			profile.WorkspaceKey,
			profile.WorkspaceName,
			profile.UserEmail,
			profile.UserName,
			profile.Label(),
		}
		for _, value := range values {
			if strings.EqualFold(value, selector) {
				matches = append(matches, profile)
				break
			}
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return auth.Profile{}, fmt.Errorf("no saved Linear account matches %q; run with --accounts", selector)
	default:
		return auth.Profile{}, fmt.Errorf("%q matches multiple Linear accounts; use the profile ID shown by --accounts", selector)
	}
}

func printProfiles(writer io.Writer, profiles []auth.Profile, activeID string) {
	if len(profiles) == 0 {
		fmt.Fprintln(writer, "No saved Linear accounts. Run tuinear --login to add one.")
		return
	}
	for _, profile := range profiles {
		marker := " "
		if profile.ID == activeID {
			marker = "*"
		}
		label := profile.Label()
		if profile.UserEmail != "" && !strings.Contains(label, profile.UserEmail) {
			label += " <" + profile.UserEmail + ">"
		}
		if profile.WorkspaceKey != "" {
			label += " [" + profile.WorkspaceKey + "]"
		}
		fmt.Fprintf(writer, "%s %s  %s\n", marker, profile.ID, label)
	}
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
