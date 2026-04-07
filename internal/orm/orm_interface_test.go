// Package orm_test is a black-box test package that can import both
// internal/orm and pkg/flow/api without introducing an import cycle.
// It is kept separate from bun_adapter_test.go (package orm) for that reason.
package orm_test

import (
	"testing"

	"github.com/undiegomejia/flow/internal/orm"
	"github.com/undiegomejia/flow/pkg/flow/api"

	_ "modernc.org/sqlite"
)

// TestBunAdapterSatisfiesAPIORMInterface is a compile-time assertion that
// *orm.BunAdapter implements the api.ORM interface. If it does not, this
// file will fail to compile.
func TestBunAdapterSatisfiesAPIORMInterface(t *testing.T) {
	var _ api.ORM = (*orm.BunAdapter)(nil)
}
