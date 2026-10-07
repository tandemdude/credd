package opwd

import (
	"context"
	"errors"
	"testing"

	"github.com/tandemdude/credd/internal/secretstore"
)

// TestRepairReferenceRejectsMalformed guards against the index-out-of-range
// panic that a missing item/field path used to trigger. The malformed cases
// must return an error before touching the (nil here) VaultManager.
func TestRepairReferenceRejectsMalformed(t *testing.T) {
	c := &Client{} // vm intentionally nil: a panic-free path must not reach it

	cases := []string{
		"op://Private",     // vault only, no item/field
		"op://",            // empty
		"op://Private/",    // empty item/field
		"op:///item/field", // empty vault
	}

	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			if _, err := c.repairReference(context.Background(), ref); !errors.Is(err, secretstore.ErrInvalidReference) {
				t.Fatalf("expected ErrInvalidReference for malformed reference %q, got %v", ref, err)
			}
		})
	}
}
