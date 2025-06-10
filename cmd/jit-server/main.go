package main

import (
	"os"

	"github.com/rebelopsio/jit-bot/cmd/jit-server/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}