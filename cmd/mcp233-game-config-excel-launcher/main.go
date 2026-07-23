// mcp233-game-config-excel-launcher starts the selected local MCP binary over stdio.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/neko233-com/mcp233-game-config-excel/internal/updater"
)

func main() {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	status, err := updater.ReadStatus(filepath.Dir(executable))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	command := exec.Command(status.SelectedPath, os.Args[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
