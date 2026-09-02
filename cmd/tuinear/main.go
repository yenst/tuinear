package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jihmy/tuinear/internal/auth"
	"github.com/jihmy/tuinear/internal/linear"
	"github.com/jihmy/tuinear/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	// oauthClientID may be supplied at build time with -ldflags -X.
	oauthClientID = ""
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("tuinear", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	demo := flags.Bool("demo", false, "use built-in sample tickets")
	snapshot := flags.Bool("snapshot", false, "print a demo screen and exit")
	login := flags.Bool("login", false, "sign in to Linear in your browser")
	logout := flags.Bool("logout", false, "revoke the saved Linear session")
	showVersion := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if (*login && *logout) || ((*login || *logout) && (*demo || *snapshot)) {
		fmt.Fprintln(os.Stderr, "Choose only one of --login, --logout, --demo, or --snapshot.")
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
	if *demo {
		loader = linear.DemoClient{}
	} else {
		apiKey := os.Getenv("LINEAR_API_KEY")
		if apiKey != "" {
			if *login || *logout {
				fmt.Fprintln(os.Stderr, "Unset LINEAR_API_KEY before using --login or --logout.")
				return 2
			}
			loader = linear.NewClient(apiKey)
		} else {
			clientID := os.Getenv("TUINEAR_OAUTH_CLIENT_ID")
			if clientID == "" {
				clientID = oauthClientID
			}
			if clientID == "" {
				fmt.Fprintln(os.Stderr, "TUINEAR_OAUTH_CLIENT_ID is not set. See README.md for one-time OAuth app setup, or run with --demo.")
				return 2
			}
			manager := auth.NewManager(clientID)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			if *logout {
				if err := manager.Logout(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Logout failed: %v\n", err)
					return 1
				}
				fmt.Println("Logged out of Linear.")
				return 0
			}
			if *login {
				if err := loginWithBrowser(ctx, manager); err != nil {
					fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
					return 1
				}
				fmt.Println("Connected to Linear.")
				return 0
			}
			if _, err := manager.Token(ctx); errors.Is(err, auth.ErrNotLoggedIn) {
				fmt.Println("No saved Linear session; starting browser login.")
				if err := loginWithBrowser(ctx, manager); err != nil {
					fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
					return 1
				}
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "Load Linear session: %v\n", err)
				return 1
			}
			loader = linear.NewOAuthClient(manager)
		}
	}

	program := tea.NewProgram(ui.New(loader))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Tuinear failed: %v\n", err)
		return 1
	}
	return 0
}

func loginWithBrowser(ctx context.Context, manager *auth.Manager) error {
	return manager.Login(ctx, func(authorizeURL string) {
		fmt.Printf("Open this URL to connect Linear:\n%s\n", authorizeURL)
		if err := auth.OpenBrowser(authorizeURL); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open a browser automatically: %v\n", err)
		}
	})
}
