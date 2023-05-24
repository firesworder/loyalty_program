package storage

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
)

var (
	ErrLoginExist        = errors.New("login already exist")
	ErrAuthDataIncorrect = errors.New("login or password incorrect")
)

type MockUser struct {
	Login          string
	HashedPassword string
	AuthToken      string
}

func (m *MockUser) GetToken() string {
	return m.AuthToken
}

func (m *MockUser) GetLogin() string {
	return m.Login
}

func (m *MockUser) GetPassword() string {
	return m.HashedPassword
}

type Mock struct {
	Users map[string]MockUser
}

func (m *Mock) Ping() error {
	return nil
}

func (m *Mock) AddUser(login, password string) (User, error) {
	if _, loginExist := m.Users[login]; loginExist {
		return nil, ErrLoginExist
	}

	hash := md5.Sum([]byte(password))
	hashedPassword := hex.EncodeToString(hash[:])
	u := MockUser{Login: login, HashedPassword: hashedPassword, AuthToken: "someAuthToken"}
	m.Users[login] = u
	return &u, nil
}

func (m *Mock) GetUser(login, password string) (User, error) {
	user, ok := m.Users[login]
	if !ok || user.HashedPassword != password {
		return nil, ErrAuthDataIncorrect
	}
	return &user, nil
}
