package storage

import (
	"errors"
	"time"
)

var (
	ErrLoginExist          = errors.New("login already exist")
	ErrAuthDataIncorrect   = errors.New("login or password incorrect")
	ErrOrderRegByThatUser  = errors.New("order already registered by you")
	ErrOrderRegByOtherUser = errors.New("order has been registered already by other user")
	ErrBalanceExceeded     = errors.New("balance exceeded")
)

type OrderStatus struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Amount     int64     `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
	UserID     int64     `json:"-"`
}

type Withdrawn struct {
	OrderID     string    `json:"order"`
	Amount      int64     `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
	UserID      int64     `json:"-"`
}

type Balance struct {
	UserId          int64
	BalanceAmount   int64
	WithdrawnAmount int64
}

type User struct {
	ID       int64
	Login    string
	Password string
}
