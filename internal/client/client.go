package client

import (
	"context"

	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"
	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the generated gRPC clients with a small, intention-revealing
// surface for the CLI.
type Client struct {
	conn     *grpc.ClientConn
	rpc      secretsv1.SecretsClient
	profiles profilesv1.ProfilesClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return newFromConn(conn), nil
}

func newFromConn(conn *grpc.ClientConn) *Client {
	return &Client{
		conn:     conn,
		rpc:      secretsv1.NewSecretsClient(conn),
		profiles: profilesv1.NewProfilesClient(conn),
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Get resolves a secret reference (e.g. op://vault/item/field) and returns
// its value.
func (c *Client) Get(ctx context.Context, ref string) (string, error) {
	resp, err := c.rpc.GetSecret(ctx, &secretsv1.GetSecretRequest{Reference: ref})
	if err != nil {
		return "", err
	}
	return resp.GetSecret(), nil
}

// Exists reports whether a secret reference points at an existing secret.
func (c *Client) Exists(ctx context.Context, ref string) (bool, error) {
	resp, err := c.rpc.SecretExists(ctx, &secretsv1.SecretExistsRequest{Reference: ref})
	if err != nil {
		return false, err
	}
	return resp.GetExists(), nil
}
