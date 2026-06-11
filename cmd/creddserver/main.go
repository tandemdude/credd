package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/tandemdude/credd/db"
	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"
	"github.com/tandemdude/credd/internal/config"
	"github.com/tandemdude/credd/internal/opwd"
	"github.com/tandemdude/credd/internal/server"
	"github.com/tandemdude/credd/internal/sqlite"

	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"

	_ "modernc.org/sqlite"
)

func newOnePasswordClient(opAccountName string) func(ctx context.Context) (server.SecretResolver, error) {
	return func(ctx context.Context) (server.SecretResolver, error) {
		c, err := opwd.NewClient(ctx, opAccountName)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
}

func openDB(path string) (*sql.DB, *db.Queries, error) {
	conn, err := sql.Open("sqlite", "file:"+path+"?_foreign_keys=on")
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	if err = db.Migrate(conn); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("migrate database: %w", err)
	}

	return conn, db.New(conn), nil
}

func runServer(ctx context.Context, addr, opAccountName, dbPath string) error {
	conn, q, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	store := sqlite.NewStore(q)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			server.RecoveryUnaryInterceptor,
			server.LoggingUnaryInterceptor,
		),
	)
	secretsv1.RegisterSecretsServer(grpcServer, server.NewSecretsServer(store.Secrets, newOnePasswordClient(opAccountName)))

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("gRPC server listening", "addr", addr)
		serveErr <- grpcServer.Serve(lis)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		slog.Info("shutting down gRPC server")
		grpcServer.GracefulStop()
		return nil
	}
}

// serverFlags builds the creddserver flags. configPath is bound to --config and
// shared with the TOML value sources so a resolved --config / CREDD_CONFIG path
// is honoured. Precedence per value: explicit flag > env var > config file > default.
func serverFlags(defaultConfigPath string, configPath *string) []cli.Flag {
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
			Usage: "address to listen on",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("CREDD_SERVER_ADDR"),
				config.TOMLSource("server.addr", configPath),
			),
		},
		&cli.StringFlag{
			Name:  "1password.account",
			Usage: "1Password account name or UUID (required for 1Password integration)",
			Sources: cli.NewValueSourceChain(
				cli.EnvVar("CREDD_1PASSWORD_ACCOUNT"),
				config.TOMLSource("1password.account", configPath),
			),
		},
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	defaultConfigPath, err := config.DefaultConfigPath()
	if err != nil {
		slog.Error("resolve config path", "err", err)
		os.Exit(1)
	}

	var configPath string
	cmd := &cli.Command{
		Name:  "creddserver",
		Usage: "start the credd gRPC server",
		Flags: serverFlags(defaultConfigPath, &configPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if err := config.Validate(configPath); err != nil {
				return err
			}
			return runServer(ctx, cmd.String("server.addr"), cmd.String("1password.account"), config.DataDBPath(configPath))
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("creddserver exited with error", "err", err)
		os.Exit(1)
	}
}
