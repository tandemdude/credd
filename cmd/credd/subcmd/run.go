package subcmd

import (
	"context"

	"github.com/tandemdude/credd/internal/client"
	"github.com/tandemdude/credd/internal/config"
	"github.com/urfave/cli/v3"
)

// dial validates the resolved config file and constructs a client from the
// --server.addr flag. Shared by the run and secret commands.
func dial(cmd *cli.Command) (*client.Client, error) {
	if err := config.Validate(cmd.String("config")); err != nil {
		return nil, err
	}
	return client.New(cmd.String("server.addr"))
}

// RunCmd runs a command with resolved secrets injected as env vars.
var RunCmd = &cli.Command{
	Name:      "run",
	Usage:     "run a command with secrets injected as env vars",
	ArgsUsage: "--env NAME=ref [--env NAME=ref ...] -- <cmd> [args...]",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "env",
			Usage: "env var in NAME=ref form (ref is a secret name or op:// reference); repeatable",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		c, err := dial(cmd)
		if err != nil {
			return err
		}
		defer c.Close()

		code, err := c.Run(ctx, cmd.StringSlice("env"), cmd.Args().Slice())
		if err != nil {
			return err
		}
		if code != 0 {
			return cli.Exit("", code)
		}
		return nil
	},
}
