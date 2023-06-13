package storage

import "context"

type Storage interface {
	AddUser(ctx context.Context, login, hashedPassword string) (*User, error)
	GetUser(ctx context.Context, login string) (*User, error)

	GetBalance(ctx context.Context, user User) (*Balance, error)
	UpdateBalance(ctx context.Context, newBalance Balance) error
	GetOrderStatusList(ctx context.Context, user User) ([]OrderStatus, error)
	AddOrder(ctx context.Context, orderNumber string, user User) error

	UpdateOrderStatuses(ctx context.Context, orderStatusList []OrderStatus) error
	GetOrdersWithTemporaryStatus(ctx context.Context) ([]OrderStatus, error)

	AddWithdrawn(ctx context.Context, orderNumber string, amount float64, user User) error
	GetWithdrawnList(ctx context.Context, user User) ([]Withdrawn, error)
}
