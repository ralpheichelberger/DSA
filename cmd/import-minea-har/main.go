package main

import (
	"fmt"
	"os"

	"github.com/dropshipagent/agent/internal/integrations/minea"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: import-minea-har <file.har> [session.json]")
		fmt.Fprintln(os.Stderr, "  Reads a Chrome DevTools HAR export, finds AppSync GraphQL POST (200), writes session.")
		fmt.Fprintln(os.Stderr, "  Default session path: ./data/minea_session.json")
		os.Exit(1)
	}
	harPath := os.Args[1]
	sessionPath := "./data/minea_session.json"
	if len(os.Args) > 2 {
		sessionPath = os.Args[2]
	}
	gqlURL, err := minea.ImportMineaHAR(harPath, sessionPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import failed:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", sessionPath)
	fmt.Fprintf(os.Stderr, "set in .env (if not already default):\n  MINEA_GRAPHQL_URL=%s\n", gqlURL)
}
