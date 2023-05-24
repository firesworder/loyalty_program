package server

import (
	"fmt"
	"github.com/firesworder/loyalty_program/internal/storage"
)

type AuthTokensCache struct {
	Users map[string]storage.User
}

func NewAuthTokensCache(users map[string]storage.User) *AuthTokensCache {
	c := AuthTokensCache{Users: map[string]storage.User{}}
	if users != nil {
		c.Users = users
	}
	return &c
}

func (c *AuthTokensCache) AddUser(authToken string, user storage.User) error {
	if _, ok := c.Users[authToken]; ok {
		return fmt.Errorf("token `%s` already in cache", authToken)
	}
	c.Users[authToken] = user
	return nil
}

func (c *AuthTokensCache) IsTokenExist(authToken string) bool {
	_, ok := c.Users[authToken]
	return ok
}
