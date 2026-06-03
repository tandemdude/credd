package opwd

import (
	"context"
	"fmt"
	"strings"

	"github.com/1password/onepassword-sdk-go"
)

func IsOpwdRef(ref string) bool {
	return strings.HasPrefix(ref, "op://")
}

type Client struct {
	op *onepassword.Client

	vm *VaultManager
}

func NewClient(ctx context.Context, accountName string) (*Client, error) {
	op, err := onepassword.NewClient(
		ctx,
		onepassword.WithDesktopAppIntegration(accountName),
		onepassword.WithIntegrationInfo("credd", "v0.0.1"),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		op: op,
		vm: newVaultManager(op),
	}, nil
}

func (c *Client) repairReference(ctx context.Context, reference string) (string, error) {
	trimmed := strings.TrimPrefix(reference, "op://")
	vaultName, rest, found := strings.Cut(trimmed, "/")
	if !found || vaultName == "" || rest == "" {
		return "", fmt.Errorf("invalid 1Password reference %q: expected op://<vault>/<item>/<field>", reference)
	}

	vaultUUID, err := c.vm.ResolveUUID(ctx, vaultName)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("op://%s/%s", vaultUUID, rest), nil
}

func (c *Client) ResolveSecret(ctx context.Context, reference string) (string, error) {
	repairedReference, err := c.repairReference(ctx, reference)
	if err != nil {
		return "", err
	}

	return c.op.Secrets().Resolve(ctx, repairedReference)
}
