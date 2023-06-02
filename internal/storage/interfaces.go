package storage

type Storage interface {
	AddUser(login, password string) (*User, error)
	GetUser(login, password string) (*User, error)

	GetBalance(user User) (int64, error)
	UpdateBalance(newAmount int64, user User) error
	GetOrderStatusList(user User) []OrderStatus
	AddOrder(orderNumber string, user User) error

	UpdateOrderStatuses(orderStatusList []OrderStatus) error
	GetOrdersWithTemporaryStatus() ([]OrderStatus, error)
	GetAllOrderStatusList() ([]OrderStatus, error)

	GetWithdrawn(user User) int64
	AddWithdrawn(orderNumber string, amount int64, user User) error
	GetWithdrawnList(user User) []Withdrawn
}
