package subcmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/tandemdude/credd/cmd/credd/util"
	"github.com/tandemdude/credd/internal/config"

	"github.com/urfave/cli/v3"
)

// runInit prompts on in, writes prompts to out, and saves a config file to path.
// An existing file must be confirmed before it is overwritten.
func runInit(in io.Reader, out io.Writer, path string) error {
	scanner := bufio.NewScanner(in)

	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(out, "config already exists at %s; overwrite? [y/N]: ", path)
		if !util.ScanYes(scanner) {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	addr := util.PromptString(scanner, out, "server address", "127.0.0.1:50051")
	account := util.PromptString(scanner, out, "1Password account (press enter to skip)", "")

	cfg := config.Config{
		Server:      config.ServerConfig{Addr: addr},
		OnePassword: config.OnePasswordConfig{Account: account},
	}
	if err := config.Write(path, cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote config to %s\n", path)
	return nil
}

// InitCmd runs the interactive wizard to create the config file.
var InitCmd = &cli.Command{
	Name:  "init",
	Usage: "create the credd config file interactively",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return runInit(os.Stdin, os.Stderr, cmd.String("config"))
	},
}
