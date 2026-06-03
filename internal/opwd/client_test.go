package opwd

import (
	"context"
	"testing"
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
			if _, err := c.repairReference(context.Background(), ref); err == nil {
				t.Fatalf("expected error for malformed reference %q, got nil", ref)
			}
		})
	}
}
