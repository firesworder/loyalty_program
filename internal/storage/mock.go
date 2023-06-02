package storage

import (
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
		UserId:     0,
	},
	{
		Number:     "order2",
		Status:     "PROCESSING",
		UploadedAt: time.Date(2023, 01, 10, 12, 0, 0, 0, time.Local),
		UserId:     1,
	},
	{
		Number:     "order3",
		Status:     "INVALID",
		UploadedAt: time.Date(2023, 02, 10, 12, 0, 0, 0, time.Local),
		UserId:     0,
	},
	{
		Number:     "order4",
		Status:     "NEW",
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
		UserId:     1,
	},
	{
		Number:     "9359943520",
		Status:     "NEW",
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
		UserId:     0,
	},
	{
		Number:     "328257446760",
		Status:     "INVALID",
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
		UserId:     1,
	},
}

var MockWithdrawnData = []Withdrawn{
	{
		OrderId:     "order6",
		Amount:      100,
		ProcessedAt: time.Date(2023, 04, 02, 12, 0, 0, 0, time.Local),
		UserId:      0,
	},
	{
		OrderId:     "order7",
		Amount:      200,
		ProcessedAt: time.Date(2023, 05, 02, 12, 0, 0, 0, time.Local),
		UserId:      1,
	},
}

var MockUserData = []User{
	{ID: 0, Login: "admin", Password: "admin"},
	{ID: 1, Login: "postgres", Password: "postgres"},
}

var MockUserBalanceData = []Balance{
	{UserId: 0, BalanceAmount: 900},
	{UserId: 1, BalanceAmount: 900},
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

func (m *Mock) AddUser(login, password string) (*User, error) {
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

func (m *Mock) GetUser(login, password string) (*User, error) {
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

func (m *Mock) GetBalance(user User) (int64, error) {
	for _, b := range m.Balance {
		if b.UserId == user.ID {
			return b.BalanceAmount, nil
		}
	}
	return 0, fmt.Errorf("user balance not defined")
}

func (m *Mock) UpdateBalance(newAmount int64, user User) error {
	for _, b := range m.Balance {
		if b.UserId == user.ID {
			b.BalanceAmount = newAmount
			return nil
		}
	}
	return fmt.Errorf("user balance not defined")
}

func (m *Mock) GetWithdrawn(user User) int64 {
	return 15
}

func (m *Mock) GetWithdrawnList(user User) []Withdrawn {
	result := make([]Withdrawn, 0)
	for _, mW := range m.Withdrawn {
		if mW.UserId == user.ID {
			result = append(result, mW)
		}
	}
	return result
}

func (m *Mock) GetOrderStatusList(user User) []OrderStatus {
	result := make([]OrderStatus, 0)
	for _, mOS := range m.OrderStatus {
		if mOS.UserId == user.ID {
			result = append(result, mOS)
		}
	}
	return result
}

func (m *Mock) AddOrder(orderNumber string, user User) error {
	for _, order := range m.OrderStatus {
		if order.Number == orderNumber {
			if order.UserId == user.ID {
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
		UserId:     user.ID,
	}
	m.OrderStatus = append(m.OrderStatus, order)
	return nil
}

func (m *Mock) AddWithdrawn(orderNumber string, amount int64, user User) error {
	curBalance, err := m.GetBalance(user)
	if err != nil {
		return err
	}

	if curBalance < amount {
		return ErrBalanceExceeded
	}

	w := Withdrawn{
		OrderId:     orderNumber,
		Amount:      amount,
		ProcessedAt: time.Now(),
		UserId:      user.ID,
	}
	m.Withdrawn = append(m.Withdrawn, w)
	newBalance := curBalance - amount
	err = m.UpdateBalance(newBalance, user)
	if err != nil {
		return err
	}

	return nil
}

func (m *Mock) GetOrdersWithTemporaryStatus() ([]OrderStatus, error) {
	result := make([]OrderStatus, 0)
	for _, oS := range m.OrderStatus {
		if oS.Status == "NEW" || oS.Status == "PROCESSING" {
			result = append(result, oS)
		}
	}
	return result, nil
}

func (m *Mock) UpdateOrderStatuses(orderStatusList []OrderStatus) error {
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

func (m *Mock) GetAllOrderStatusList() ([]OrderStatus, error) {
	return m.OrderStatus, nil
}
