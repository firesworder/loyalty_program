package storage

type Storage interface {
	Ping() error
	AddUser(login, password string) (User, error)
	GetUser(login, password string) (User, error)

	GetBalance(user User) (float64, error)
	GetOrderStatusList(user User) []MockOrderStatus
	AddOrder(orderNumber string, user User) error

	GetWithdrawn(user User) int64
	AddWithdrawn(orderNumber string, amount int64, user User) error
	GetWithdrawnList(user User) []MockWithdrawn

	ResetData()
}

type User interface {
	GetToken() string
	GetLogin() string
	GetPassword() string
	GetId() int64
}
