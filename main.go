package main

import (
	"github.com/glassnode/glassnode-cli/cmd"
	"github.com/glassnode/glassnode-cli/internal/version"
)

func main() {
	cmd.SetVersion(version.Version)
	cmd.Execute()
}
