package subcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

// SecretCmd looks up secrets in the configured external secret managers.
var SecretCmd = &cli.Command{
	Name:  "secret",
	Usage: "look up secrets in external secret managers",
	Commands: []*cli.Command{
		{
			Name:      "show",
			Usage:     "print a secret's value",
			ArgsUsage: "<ref>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd secret show <ref>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				value, err := c.Get(ctx, cmd.Args().Get(0))
				if err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, value)
				return nil
			},
		},
		{
			Name:      "exists",
			Usage:     "check whether a secret exists (exit status 0 if it does, 1 if not)",
			ArgsUsage: "<ref>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd secret exists <ref>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				ref := cmd.Args().Get(0)
				exists, err := c.Exists(ctx, ref)
				if err != nil {
					return err
				}
				if !exists {
					return cli.Exit(fmt.Sprintf("secret %q does not exist", ref), 1)
				}
				fmt.Fprintf(os.Stderr, "secret %q exists\n", ref)
				return nil
			},
		},
	},
}
