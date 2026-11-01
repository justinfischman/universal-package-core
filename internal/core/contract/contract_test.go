package contract_test

import (
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core/contract"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

// The fakes must satisfy the contract; this also proves the suites run.
func TestFakesSatisfyContract(t *testing.T) {
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	contract.RunAll(t, regs)
}
