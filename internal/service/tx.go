package service

import "context"

//Transactionmanager интерфейс для управления транзакциями
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
