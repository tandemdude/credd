package subcmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/tandemdude/credd/cmd/credd/util"
	"github.com/tandemdude/credd/internal/config"

	"github.com/urfave/cli/v3"
)

// runInit prompts using scanner, writes prompts to out, and saves a config file
// to path. It returns true if the config was written (false if an existing
// file's overwrite was declined).
func runInit(scanner *bufio.Scanner, out io.Writer, path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(out, "config already exists at %s; overwrite? [y/N]: ", path)
		if !util.ScanYes(scanner) {
			fmt.Fprintln(out, "aborted")
			return false, nil
		}
	}

	addr := util.PromptString(scanner, out, "server address", "127.0.0.1:50051")
	account := util.PromptString(scanner, out, "1Password account (press enter to skip)", "")

	cfg := config.Config{
		Server:      config.ServerConfig{Addr: addr},
		OnePassword: config.OnePasswordConfig{Account: account},
	}
	if err := config.Write(path, cfg); err != nil {
		return false, err
	}
	fmt.Fprintf(out, "wrote config to %s\n", path)
	return true, nil
}

// promptInstallService asks whether to run creddserver at login and, if
// accepted, calls install with the config path. install is injected for testing.
func promptInstallService(scanner *bufio.Scanner, out io.Writer, configPath string, install func(string) error) error {
	fmt.Fprint(out, "Run creddserver automatically at login? [y/N]: ")
	if !util.ScanYes(scanner) {
		return nil
	}
	if err := install(configPath); err != nil {
		return err
	}
	fmt.Fprintln(out, "creddserver will now run automatically at login")
	return nil
}

// InitCmd runs the interactive wizard to create the config file and optionally
// install the creddserver login service.
var InitCmd = &cli.Command{
	Name:  "init",
	Usage: "create the credd config file interactively",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		scanner := bufio.NewScanner(os.Stdin)
		out := os.Stderr
		configPath := cmd.String("config")

		wrote, err := runInit(scanner, out, configPath)
		if err != nil {
			return err
		}
		if !wrote || runtime.GOOS != "darwin" {
			return nil
		}
		return promptInstallService(scanner, out, configPath, installService)
	},
}
