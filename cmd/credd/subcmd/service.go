package subcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/tandemdude/credd/internal/launchd"

	"github.com/urfave/cli/v3"
)

// installService resolves the creddserver binary and installs the LaunchAgent
// for the given config path. Shared by ServiceCmd and the init prompt.
func installService(configPath string) error {
	m, err := launchd.NewManager()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve own path: %w", err)
	}
	serverPath, err := launchd.ResolveServerPath(self)
	if err != nil {
		return err
	}
	return m.Install(serverPath, configPath)
}

// ServiceCmd manages the creddserver login service (macOS LaunchAgent).
var ServiceCmd = &cli.Command{
	Name:  "service",
	Usage: "manage the creddserver login service (macOS)",
	Commands: []*cli.Command{
		{
			Name:  "install",
			Usage: "install and start the creddserver LaunchAgent",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if err := installService(cmd.String("config")); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "creddserver installed; it will run at login and has been started")
				return nil
			},
		},
		{
			Name:  "uninstall",
			Usage: "stop and remove the creddserver LaunchAgent",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				m, err := launchd.NewManager()
				if err != nil {
					return err
				}
				if err := m.Uninstall(); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "creddserver service removed")
				return nil
			},
		},
		{
			Name:  "start",
			Usage: "start the creddserver LaunchAgent",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				m, err := launchd.NewManager()
				if err != nil {
					return err
				}
				if err := m.Start(); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "creddserver started")
				return nil
			},
		},
		{
			Name:  "stop",
			Usage: "stop the creddserver LaunchAgent",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				m, err := launchd.NewManager()
				if err != nil {
					return err
				}
				if err := m.Stop(); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "creddserver stopped")
				return nil
			},
		},
		{
			Name:  "restart",
			Usage: "restart the creddserver LaunchAgent",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				m, err := launchd.NewManager()
				if err != nil {
					return err
				}
				if err := m.Restart(); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "creddserver restarted")
				return nil
			},
		},
		{
			Name:  "status",
			Usage: "show the creddserver LaunchAgent status",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				m, err := launchd.NewManager()
				if err != nil {
					return err
				}
				st, err := m.Status()
				if err != nil {
					return err
				}
				if !st.Installed {
					fmt.Fprintln(os.Stdout, "not installed")
					return nil
				}
				if !st.Loaded {
					fmt.Fprintln(os.Stdout, "installed, not running")
					return nil
				}
				fmt.Fprintf(os.Stdout, "running (pid %d, last exit %d)\n", st.PID, st.LastExit)
				return nil
			},
		},
	},
}
