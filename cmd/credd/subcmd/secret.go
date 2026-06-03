package subcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/tandemdude/credd/cmd/credd/util"

	"github.com/urfave/cli/v3"
)

// SecretCmd manages stored secrets (add/show/list/delete/deleteall).
var SecretCmd = &cli.Command{
	Name:  "secret",
	Usage: "manage stored secrets",
	Commands: []*cli.Command{
		{
			Name:      "add",
			Usage:     "store a secret",
			ArgsUsage: "<name> <value>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 2 {
					return cli.Exit("usage: credd secret add <name> <value>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				name, value := cmd.Args().Get(0), cmd.Args().Get(1)
				if err := c.Add(ctx, name, value); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "stored secret %q\n", name)
				return nil
			},
		},
		{
			Name:      "show",
			Usage:     "print a secret's value",
			ArgsUsage: "<name|op://ref>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd secret show <name>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				value, err := c.Show(ctx, cmd.Args().Get(0))
				if err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, value)
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "list secret names",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				names, err := c.List(ctx)
				if err != nil {
					return err
				}
				for _, n := range names {
					fmt.Fprintln(os.Stdout, n)
				}
				return nil
			},
		},
		{
			Name:      "delete",
			Usage:     "delete a secret",
			ArgsUsage: "<name>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd secret delete <name>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				name := cmd.Args().Get(0)
				if err := c.Delete(ctx, name); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "deleted secret %q\n", name)
				return nil
			},
		},
		{
			Name:  "deleteall",
			Usage: "delete ALL secrets",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "yes",
					Aliases: []string{"y"},
					Usage:   "skip the confirmation prompt",
				},
			},
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if !cmd.Bool("yes") && !util.Confirm("Delete ALL secrets? [y/N]: ") {
					fmt.Fprintln(os.Stderr, "aborted")
					return nil
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				if err := c.DeleteAll(ctx); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "deleted all secrets")
				return nil
			},
		},
	},
}
