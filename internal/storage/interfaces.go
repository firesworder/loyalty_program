package storage

type Storage interface {
	Ping() error
	AddUser(login, password string) (User, error)
	GetUser(login, password string) (User, error)
}

type User interface {
	GetToken() string
	GetLogin() string
	GetPassword() string
}
