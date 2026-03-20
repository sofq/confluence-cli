package main

import (
	"os"

	"github.com/sofq/confluence-cli/cmd"
)

func main() {
	code := cmd.Execute()
	os.Exit(code)
}
