package main

import (
	"os"

	"github.com/ytyng/gh-copilot-review/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:]))
}
