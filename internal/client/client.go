package client

import (
	"context"

	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the generated gRPC SecretsClient with a small, intention-
// revealing surface for the CLI.
type Client struct {
	conn *grpc.ClientConn
	rpc  secretsv1.SecretsClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, rpc: secretsv1.NewSecretsClient(conn)}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Add stores, or updates a secret under name.
func (c *Client) Add(ctx context.Context, name, value string) error {
	_, err := c.rpc.CreateSecret(ctx, &secretsv1.CreateSecretRequest{Name: name, Value: value})
	return err
}

// Show resolves a secret by name or 1Password reference and returns its value.
func (c *Client) Show(ctx context.Context, ref string) (string, error) {
	resp, err := c.rpc.GetSecret(ctx, &secretsv1.GetSecretRequest{Reference: ref})
	if err != nil {
		return "", err
	}
	return resp.GetSecret(), nil
}

// List returns the names of all stored secrets.
func (c *Client) List(ctx context.Context) ([]string, error) {
	resp, err := c.rpc.ListSecrets(ctx, &secretsv1.ListSecretsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetName(), nil
}

// Delete removes a single secret by name.
func (c *Client) Delete(ctx context.Context, name string) error {
	_, err := c.rpc.DeleteSecret(ctx, &secretsv1.DeleteSecretRequest{Name: name})
	return err
}

// DeleteAll removes every stored secret.
func (c *Client) DeleteAll(ctx context.Context) error {
	_, err := c.rpc.DeleteAllSecrets(ctx, &secretsv1.DeleteAllSecretsRequest{})
	return err
}
