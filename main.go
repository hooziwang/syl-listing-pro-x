package main

import (
	"os"

	"github.com/hooziwang/syl-listing-pro-x/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
