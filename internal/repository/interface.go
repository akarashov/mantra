// Package repository определяет интерфейс для работы с данными пользователей и встреч (создание, получение, обновление, поиск)
package repository

import (
	"context"
)

// Crud дженерик-интерфейс для работы с данными
type Crud[T any, ID any] interface {
	Create(ctx context.Context, v *T) (*T, error)
	Read(ctx context.Context, id ID) (*T, error)
	Update(ctx context.Context, id ID, v *T) error
}

// Searcher дженерик-интерфейс для получения списков и полнотекстового поиска.
type Searcher[T any, LQ any, SQ any, R any] interface {
	List(ctx context.Context, q LQ) ([]T, error)
	Search(ctx context.Context, q SQ) ([]R, error)
}
