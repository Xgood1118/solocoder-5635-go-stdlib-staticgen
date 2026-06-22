package main

import (
	"fmt"
	"os"

	"github.com/techblog/staticgen/pkg/cli"
)

func main() {
	c := cli.New()
	if err := c.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
