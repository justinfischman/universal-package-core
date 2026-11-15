package runstate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const providerFile = "provider"

// ProviderFile is the record of which up provider started the app, so down
// reaches the same provider.
func (s State) ProviderFile() string { return filepath.Join(s.RunDir(), providerFile) }

// SetProvider records that the named up provider started the app.
func (s State) SetProvider(name string) error {
	if err := os.MkdirAll(s.RunDir(), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", s.RunDir(), err)
	}
	if err := os.WriteFile(s.ProviderFile(), []byte(name+"\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.ProviderFile(), err)
	}
	return nil
}

// Provider returns the recorded up provider, or "" when none started the app.
func (s State) Provider() (string, error) {
	b, err := os.ReadFile(s.ProviderFile())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", s.ProviderFile(), err)
	}
	return strings.TrimSpace(string(b)), nil
}

// ClearProvider removes the provider record; a missing record is fine.
func (s State) ClearProvider() error {
	if err := os.Remove(s.ProviderFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", s.ProviderFile(), err)
	}
	return nil
}
