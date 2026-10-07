package opwd

import (
	"context"
	"fmt"
	"strings"

	"github.com/1password/onepassword-sdk-go"

	"github.com/tandemdude/credd/internal/secretstore"
)

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
	trimmed := strings.TrimPrefix(reference, scheme+"://")
	vaultName, rest, found := strings.Cut(trimmed, "/")
	if !found || vaultName == "" || rest == "" {
		return "", fmt.Errorf("%w %q: expected op://<vault>/<item>/<field>", secretstore.ErrInvalidReference, reference)
	}

	vaultUUID, err := c.vm.ResolveUUID(ctx, vaultName)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("op://%s/%s", vaultUUID, rest), nil
}

// ResolveSecret resolves a 1Password reference to its value. Missing vaults,
// items and fields are reported as secretstore.ErrNotFound, and malformed
// references as secretstore.ErrInvalidReference.
func (c *Client) ResolveSecret(ctx context.Context, reference string) (string, error) {
	repairedReference, err := c.repairReference(ctx, reference)
	if err != nil {
		return "", err
	}

	// ResolveAll (unlike Resolve) reports per-reference failures as typed
	// errors, which lets us tell "does not exist" apart from other failures.
	resp, err := c.op.Secrets().ResolveAll(ctx, []string{repairedReference})
	if err != nil {
		return "", err
	}

	result, ok := resp.IndividualResponses[repairedReference]
	if !ok {
		return "", fmt.Errorf("1Password returned no result for %q", reference)
	}
	if result.Error != nil {
		return "", classifyResolveError(reference, *result.Error)
	}
	if result.Content == nil {
		return "", fmt.Errorf("1Password returned an empty result for %q", reference)
	}
	return result.Content.Secret, nil
}

func classifyResolveError(reference string, e onepassword.ResolveReferenceError) error {
	switch e.Type {
	case onepassword.ResolveReferenceErrorTypeVariantVaultNotFound,
		onepassword.ResolveReferenceErrorTypeVariantItemNotFound,
		onepassword.ResolveReferenceErrorTypeVariantFieldNotFound,
		onepassword.ResolveReferenceErrorTypeVariantNoMatchingSections:
		return fmt.Errorf("%w: %q (%s)", secretstore.ErrNotFound, reference, e.Type)
	case onepassword.ResolveReferenceErrorTypeVariantParsing:
		return fmt.Errorf("%w %q: %s", secretstore.ErrInvalidReference, reference, e.Parsing())
	default:
		return fmt.Errorf("failed to resolve %q: %s", reference, e.Type)
	}
}
