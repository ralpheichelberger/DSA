package main

import (
	"log"
	"os"

	"github.com/dropshipagent/agent/config"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.DevMode {
		log.Println("Agent starting in DEV_MODE")
	} else {
		log.Println("Agent starting")
	}

	os.Exit(0)
}
