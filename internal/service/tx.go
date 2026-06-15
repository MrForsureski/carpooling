package service

import "context"

// TransactionManager — интерфейс для управления транзакциями бизнес-логики
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
