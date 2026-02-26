package api

import (
	"context"
)

// BaseModel is the minimal model structure with common fields. Adapters
// (bun/gorm) may embed or map these fields as appropriate.
type BaseModel struct {
	ID int64
}

// ORM defines a small set of operations the framework expects from an ORM
// adapter. Implementations should honor the provided context for timeouts
// and cancellation.
type ORM interface {
	Insert(ctx context.Context, dst interface{}) error
	Update(ctx context.Context, dst interface{}) error
	Delete(ctx context.Context, dst interface{}) error
	FindByPK(ctx context.Context, dst interface{}, id interface{}) error
}

// Repository is a lightweight abstraction for data access used by
// generated code and examples. It is intentionally small and adapter-agnostic.
type Repository interface {
	Insert(ctx context.Context, m interface{}) error
	Update(ctx context.Context, m interface{}) error
	Delete(ctx context.Context, m interface{}) error
	FindByPK(ctx context.Context, dst interface{}, id interface{}) error
}
