package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tandemdude/credd/cmd/credd/subcmd"
	"github.com/tandemdude/credd/internal/config"

	"github.com/urfave/cli/v3"
)

// clientFlags builds the credd client flags. See serverFlags in creddserver for
// the precedence model (flag > env > config file > default). configPath is bound
// to --config and shared with the TOML value source so the resolved path is
// honoured when loading server.addr from the config file.
func clientFlags(defaultConfigPath string, configPath *string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Value:       defaultConfigPath,
			Usage:       "path to the credd config file",
			Destination: configPath,
			Sources:     cli.EnvVars("CREDD_CONFIG"),
		},
		&cli.StringFlag{
			Name:  "server.addr",
			Value: "127.0.0.1:50051",
			Usage: "credd server address",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("CREDD_SERVER_ADDR"),
				config.TOMLSource("server.addr", configPath),
			),
		},
	}
}

func main() {
	defaultConfigPath, err := config.DefaultConfigPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	var configPath string
	cmd := &cli.Command{
		Name:  "credd",
		Usage: "credd secrets client",
		Flags: clientFlags(defaultConfigPath, &configPath),
		Commands: []*cli.Command{
			subcmd.InitCmd,
			subcmd.RunCmd,
			subcmd.SecretCmd,
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
