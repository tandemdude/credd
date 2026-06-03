package opwd

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/1password/onepassword-sdk-go"
)

var ErrVaultNotExist = errors.New("vault does not exist")

var knownRemaps = map[string]string{
	"Employee": "Private",
}

type VaultManager struct {
	op *onepassword.Client

	mu    sync.Mutex
	cache map[string]string
}

func newVaultManager(op *onepassword.Client) *VaultManager {
	return &VaultManager{
		op:    op,
		cache: make(map[string]string),
	}
}

func (vm *VaultManager) ResolveUUID(ctx context.Context, vault string) (string, error) {
	if remap, ok := knownRemaps[vault]; ok {
		vault = remap
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	if uuid, ok := vm.cache[vault]; ok {
		return uuid, nil
	}

	vaults, err := vm.op.Vaults().List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list vaults: %w", err)
	}

	for _, foundVault := range vaults {
		vm.cache[foundVault.Title] = foundVault.ID
	}

	if uuid, ok := vm.cache[vault]; ok {
		return uuid, nil
	}

	return "", ErrVaultNotExist
}
