package storage

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"
)

type Mock struct {
	Users       []User
	OrderStatus []OrderStatus
	Withdrawn   []Withdrawn
	Balance     []Balance
}

var MockOrderStatusData = []OrderStatus{
	{
		Number:     "order1",
		Status:     "PROCESSED",
		Amount:     100,
		UploadedAt: time.Date(2022, 12, 10, 12, 0, 0, 0, time.Local),
		UserID:     0,
	},
	{
		Number:     "order2",
		Status:     "PROCESSING",
		UploadedAt: time.Date(2023, 01, 10, 12, 0, 0, 0, time.Local),
		UserID:     1,
	},
	{
		Number:     "order3",
		Status:     "INVALID",
		UploadedAt: time.Date(2023, 02, 10, 12, 0, 0, 0, time.Local),
		UserID:     0,
	},
	{
		Number:     "order4",
		Status:     "NEW",
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
		UserID:     1,
	},
	{
		Number:     "9359943520",
		Status:     "NEW",
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
		UserID:     0,
	},
	{
		Number:     "328257446760",
		Status:     "INVALID",
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
		UserID:     1,
	},
}

var MockWithdrawnData = []Withdrawn{
	{
		OrderID:     "order6",
		Amount:      100,
		ProcessedAt: time.Date(2023, 04, 02, 12, 0, 0, 0, time.Local),
		UserID:      0,
	},
	{
		OrderID:     "order7",
		Amount:      200,
		ProcessedAt: time.Date(2023, 05, 02, 12, 0, 0, 0, time.Local),
		UserID:      1,
	},
}

var MockUserData = []User{
	{ID: 0, Login: "admin", Password: "admin"},
	{ID: 1, Login: "postgres", Password: "postgres"},
}

var MockUserBalanceData = []Balance{
	{UserID: 0, BalanceAmount: 900, WithdrawnAmount: 15},
	{UserID: 1, BalanceAmount: 900, WithdrawnAmount: 100},
}

func NewMock() *Mock {
	m := Mock{}
	m.Users = MockUserData
	m.OrderStatus = MockOrderStatusData
	m.Withdrawn = MockWithdrawnData
	m.Balance = MockUserBalanceData
	return &m
}

func (m *Mock) ResetData() {
	m.Users = MockUserData
	m.OrderStatus = MockOrderStatusData
	m.Withdrawn = MockWithdrawnData
	m.Balance = MockUserBalanceData
}

func (m *Mock) AddUser(ctx context.Context, login, password string) (*User, error) {
	for _, user := range m.Users {
		if user.Login == login {
			return nil, ErrLoginExist
		}
	}

	hash := md5.Sum([]byte(password))
	hashedPassword := hex.EncodeToString(hash[:])
	newID := int64(len(m.Users))
	u := User{ID: newID, Login: login, Password: hashedPassword}
	m.Users = append(m.Users, u)
	return &u, nil
}

func (m *Mock) GetUser(ctx context.Context, login, password string) (*User, error) {
	for _, user := range m.Users {
		if user.Login == login {
			if user.Password == password {
				return &user, nil
			} else {
				return nil, ErrAuthDataIncorrect
			}
		}
	}
	return nil, ErrAuthDataIncorrect
}

func (m *Mock) GetBalance(ctx context.Context, user User) (*Balance, error) {
	for _, b := range m.Balance {
		if b.UserID == user.ID {
			return &b, nil
		}
	}
	return nil, fmt.Errorf("user balance not defined")
}

func (m *Mock) UpdateBalance(ctx context.Context, newBalance Balance) error {
	for i := range m.Balance {
		if m.Balance[i].UserID == newBalance.UserID {
			m.Balance[i] = newBalance
			return nil
		}
	}
	return fmt.Errorf("user balance not defined")
}

func (m *Mock) GetWithdrawnList(ctx context.Context, user User) []Withdrawn {
	result := make([]Withdrawn, 0)
	for _, mW := range m.Withdrawn {
		if mW.UserID == user.ID {
			result = append(result, mW)
		}
	}
	return result
}

func (m *Mock) GetOrderStatusList(ctx context.Context, user User) []OrderStatus {
	result := make([]OrderStatus, 0)
	for _, mOS := range m.OrderStatus {
		if mOS.UserID == user.ID {
			result = append(result, mOS)
		}
	}
	return result
}

func (m *Mock) AddOrder(ctx context.Context, orderNumber string, user User) error {
	for _, order := range m.OrderStatus {
		if order.Number == orderNumber {
			if order.UserID == user.ID {
				return ErrOrderRegByThatUser
			} else {
				return ErrOrderRegByOtherUser
			}
		}
	}

	order := OrderStatus{
		Number:     orderNumber,
		Status:     "NEW",
		UploadedAt: time.Now(),
		UserID:     user.ID,
	}
	m.OrderStatus = append(m.OrderStatus, order)
	return nil
}

func (m *Mock) AddWithdrawn(ctx context.Context, orderNumber string, amount int64, user User) error {
	curBalance, err := m.GetBalance(ctx, user)
	if err != nil {
		return err
	}

	if curBalance.BalanceAmount < amount {
		return ErrBalanceExceeded
	}

	w := Withdrawn{
		OrderID:     orderNumber,
		Amount:      amount,
		ProcessedAt: time.Now(),
		UserID:      user.ID,
	}
	m.Withdrawn = append(m.Withdrawn, w)
	newBalance := Balance{
		UserID:          user.ID,
		BalanceAmount:   curBalance.BalanceAmount - amount,
		WithdrawnAmount: curBalance.WithdrawnAmount + amount,
	}
	err = m.UpdateBalance(ctx, newBalance)
	if err != nil {
		return err
	}

	return nil
}

func (m *Mock) GetOrdersWithTemporaryStatus(ctx context.Context) ([]OrderStatus, error) {
	result := make([]OrderStatus, 0)
	for _, oS := range m.OrderStatus {
		if oS.Status == "NEW" || oS.Status == "PROCESSING" {
			result = append(result, oS)
		}
	}
	return result, nil
}

func (m *Mock) UpdateOrderStatuses(ctx context.Context, orderStatusList []OrderStatus) error {
	for _, uOS := range orderStatusList {
		for cOSIndex := range m.OrderStatus {
			if uOS.Number == m.OrderStatus[cOSIndex].Number {
				m.OrderStatus[cOSIndex].Status = uOS.Status
				m.OrderStatus[cOSIndex].Amount = uOS.Amount
			}
		}
	}
	return nil
}
