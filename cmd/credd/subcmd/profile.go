package subcmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"
	"github.com/tandemdude/credd/internal/client"

	"github.com/urfave/cli/v3"
)

// kindName renders a var kind for display.
func kindName(k profilesv1.VarKind) string {
	switch k {
	case profilesv1.VarKind_VAR_KIND_SECRET:
		return "secret"
	case profilesv1.VarKind_VAR_KIND_PLAIN:
		return "plain"
	default:
		return "unknown"
	}
}

// descriptionFlag sets a profile's short description.
var descriptionFlag = &cli.StringFlag{
	Name:    "description",
	Aliases: []string{"d"},
	Usage:   "short, single-line description of what the profile is for",
}

// ProfileCmd manages profiles: named collections of env vars for `credd run`.
var ProfileCmd = &cli.Command{
	Name:  "profile",
	Usage: "manage profiles (named collections of env vars for credd run)",
	Commands: []*cli.Command{
		{
			Name:      "create",
			Usage:     "create an empty profile",
			ArgsUsage: "[--description TEXT] <profile>",
			Flags:     []cli.Flag{descriptionFlag},
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd profile create [--description TEXT] <profile>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				name := cmd.Args().Get(0)
				if err := c.CreateProfile(ctx, name, cmd.String("description")); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "created profile %q\n", name)
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "list profiles and their descriptions",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				profiles, err := c.ListProfiles(ctx)
				if err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, p := range profiles {
					fmt.Fprintf(w, "%s\t%s\n", p.GetName(), p.GetDescription())
				}
				return w.Flush()
			},
		},
		{
			Name:      "describe",
			Usage:     "set a profile's description (an empty description clears it)",
			ArgsUsage: "<profile> <description>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 2 {
					return cli.Exit("usage: credd profile describe <profile> <description>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				name := cmd.Args().Get(0)
				if err := c.SetProfileDescription(ctx, name, cmd.Args().Get(1)); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "updated description of profile %q\n", name)
				return nil
			},
		},
		{
			Name:      "show",
			Usage:     "show a profile's vars (secret references are shown, not their values)",
			ArgsUsage: "<profile>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd profile show <profile>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				p, err := c.GetProfile(ctx, cmd.Args().Get(0))
				if err != nil {
					return err
				}
				if d := p.GetDescription(); d != "" {
					fmt.Fprintf(os.Stdout, "%s\n\n", d)
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, v := range p.GetVars() {
					fmt.Fprintf(w, "%s\t%s\t%s\n", v.GetName(), kindName(v.GetKind()), v.GetValue())
				}
				return w.Flush()
			},
		},
		{
			Name:      "set",
			Usage:     "set one or more vars in a profile; values that are secret references (e.g. op://...) or templates containing them are resolved at run time, anything else is used verbatim",
			ArgsUsage: "<profile> NAME=value [NAME=value ...]",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() < 2 {
					return cli.Exit("usage: credd profile set <profile> NAME=value [NAME=value ...]", 2)
				}
				profile, specs := cmd.Args().Get(0), cmd.Args().Slice()[1:]

				// Validate every spec before making any changes.
				type pair struct{ name, value string }
				pairs := make([]pair, len(specs))
				for i, spec := range specs {
					name, value, found := strings.Cut(spec, "=")
					if !found || name == "" {
						return cli.Exit(fmt.Sprintf("invalid var %q: expected NAME=value", spec), 2)
					}
					pairs[i] = pair{name, value}
				}

				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				for _, p := range pairs {
					v, err := c.SetProfileVar(ctx, profile, p.name, p.value)
					if err != nil {
						return err
					}
					fmt.Fprintf(os.Stderr, "set %s (%s) in profile %q\n", v.GetName(), kindName(v.GetKind()), profile)
				}
				return nil
			},
		},
		{
			Name:      "unset",
			Usage:     "remove one or more vars from a profile",
			ArgsUsage: "<profile> NAME [NAME ...]",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() < 2 {
					return cli.Exit("usage: credd profile unset <profile> NAME [NAME ...]", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				profile := cmd.Args().Get(0)
				for _, name := range cmd.Args().Slice()[1:] {
					if err := c.UnsetProfileVar(ctx, profile, name); err != nil {
						return err
					}
					fmt.Fprintf(os.Stderr, "unset %s in profile %q\n", name, profile)
				}
				return nil
			},
		},
		{
			Name:      "delete",
			Usage:     "delete a profile and all of its vars",
			ArgsUsage: "<profile>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd profile delete <profile>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				name := cmd.Args().Get(0)
				if err := c.DeleteProfile(ctx, name); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "deleted profile %q\n", name)
				return nil
			},
		},
		{
			Name:      "export",
			Usage:     "export a profile as JSON for sharing (secret references are exported, never their values)",
			ArgsUsage: "[--output FILE] <profile>",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "file to write to (default: stdout)",
				},
			},
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd profile export [--output FILE] <profile>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				name := cmd.Args().Get(0)
				f, err := c.ExportProfile(ctx, name)
				if err != nil {
					return err
				}

				out := cmd.String("output")
				if out == "" || out == "-" {
					return client.WriteProfileFile(os.Stdout, f)
				}
				file, err := os.Create(out)
				if err != nil {
					return err
				}
				if err := client.WriteProfileFile(file, f); err != nil {
					file.Close()
					return err
				}
				if err := file.Close(); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "exported profile %q to %s\n", name, out)
				return nil
			},
		},
		{
			Name:      "import",
			Usage:     "create a profile from a JSON file produced by export (use - to read from stdin)",
			ArgsUsage: "[--name NAME] [--description TEXT] [--overwrite] <file>",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "name",
					Aliases: []string{"n"},
					Usage:   "import under this name instead of the one in the file",
				},
				&cli.StringFlag{
					Name:    descriptionFlag.Name,
					Aliases: descriptionFlag.Aliases,
					Usage:   "use this description instead of the one in the file",
				},
				&cli.BoolFlag{
					Name:  "overwrite",
					Usage: "replace an existing profile with the same name (its vars are discarded)",
				},
			},
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd profile import [--name NAME] [--description TEXT] [--overwrite] <file>", 2)
				}

				path := cmd.Args().Get(0)
				in := os.Stdin
				if path != "-" {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()
					in = file
				}
				f, err := client.ReadProfileFile(in)
				if err != nil {
					return err
				}
				if cmd.IsSet("name") {
					f.Name = cmd.String("name")
				}
				if cmd.IsSet("description") {
					f.Description = cmd.String("description")
				}

				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				p, err := c.ImportProfile(ctx, f, cmd.Bool("overwrite"))
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "imported profile %q with %d var(s)\n", p.GetName(), len(p.GetVars()))
				return nil
			},
		},
		{
			Name:      "validate",
			Usage:     "check that every secret referenced by a profile exists (exit status 1 if any do not)",
			ArgsUsage: "<profile>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.NArg() != 1 {
					return cli.Exit("usage: credd profile validate <profile>", 2)
				}
				c, err := dial(cmd)
				if err != nil {
					return err
				}
				defer c.Close()

				name := cmd.Args().Get(0)
				problems, err := c.ValidateProfile(ctx, name)
				if err != nil {
					return err
				}
				for _, p := range problems {
					if p.Ref != "" {
						fmt.Fprintf(os.Stderr, "%s: %s: %s\n", p.Var, p.Ref, p.Reason)
					} else {
						fmt.Fprintf(os.Stderr, "%s: %s\n", p.Var, p.Reason)
					}
				}
				if len(problems) > 0 {
					return cli.Exit(fmt.Sprintf("profile %q has %d problem(s)", name, len(problems)), 1)
				}
				fmt.Fprintf(os.Stderr, "profile %q is valid\n", name)
				return nil
			},
		},
	},
}
