package storage

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type MockUser struct {
	Id             int64
	Login          string
	HashedPassword string
	AuthToken      string
}

type MockBalance struct {
	UserId int64
	Amount float64
}

type MockWithdrawn struct {
	OrderId     string    `json:"order"`
	Amount      int64     `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
	UserId      int64     `json:"-"`
}

type MockOrderStatus struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Amount     int64     `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
	UserId     int64     `json:"-"`
}

type Mock struct {
	Users       []MockUser
	OrderStatus []MockOrderStatus
	Withdrawn   []MockWithdrawn
	Balance     []MockBalance
}

var (
	ErrLoginExist          = errors.New("login already exist")
	ErrAuthDataIncorrect   = errors.New("login or password incorrect")
	ErrOrderRegByThatUser  = errors.New("order already registered by you")
	ErrOrderRegByOtherUser = errors.New("order has been registered already by other user")
	ErrBalanceExceeded     = errors.New("balance exceeded")
)

var MockOrderStatusData = []MockOrderStatus{
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

var MockWithdrawnData = []MockWithdrawn{
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

var MockUserData = []MockUser{
	{Id: 0, Login: "admin", HashedPassword: "admin", AuthToken: "adminToken"},
	{Id: 1, Login: "postgres", HashedPassword: "postgres", AuthToken: "postgresToken"},
}

var MockUserBalanceData = []MockBalance{
	{UserId: 0, Amount: 900},
	{UserId: 1, Amount: 900},
}

func NewMock() *Mock {
	m := Mock{}
	m.Users = MockUserData
	m.OrderStatus = MockOrderStatusData
	m.Withdrawn = MockWithdrawnData
	m.Balance = MockUserBalanceData
	return &m
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

func (m *MockUser) GetId() int64 {
	return m.Id
}

func (m *Mock) AddUser(login, password string) (User, error) {
	for _, user := range m.Users {
		if user.Login == login {
			return nil, ErrLoginExist
		}
	}

	hash := md5.Sum([]byte(password))
	hashedPassword := hex.EncodeToString(hash[:])
	u := MockUser{Login: login, HashedPassword: hashedPassword, AuthToken: "someAuthToken"}
	m.Users = append(m.Users, u)
	return &u, nil
}

func (m *Mock) GetUser(login, password string) (User, error) {
	for _, user := range m.Users {
		if user.Login == login {
			if user.HashedPassword == password {
				return &user, nil
			} else {
				return nil, ErrAuthDataIncorrect
			}
		}
	}
	return nil, ErrAuthDataIncorrect
}

func (m *Mock) GetBalance(user User) (float64, error) {
	for _, b := range m.Balance {
		if b.UserId == user.GetId() {
			return b.Amount, nil
		}
	}
	return 0, fmt.Errorf("user balance not defined")
}

func (m *Mock) UpdateBalance(newAmount float64, user User) error {
	for _, b := range m.Balance {
		if b.UserId == user.GetId() {
			b.Amount = newAmount
			return nil
		}
	}
	return fmt.Errorf("user balance not defined")
}

func (m *Mock) GetWithdrawn(user User) int64 {
	return 15
}

func (m *Mock) GetWithdrawnList(user User) []MockWithdrawn {
	result := make([]MockWithdrawn, 0)
	for _, mW := range m.Withdrawn {
		if mW.UserId == user.GetId() {
			result = append(result, mW)
		}
	}
	return result
}

func (m *Mock) GetOrderStatusList(user User) []MockOrderStatus {
	result := make([]MockOrderStatus, 0)
	for _, mOS := range m.OrderStatus {
		if mOS.UserId == user.GetId() {
			result = append(result, mOS)
		}
	}
	return result
}

func (m *Mock) AddOrder(orderNumber string, user User) error {
	for _, order := range m.OrderStatus {
		if order.Number == orderNumber {
			if order.UserId == user.GetId() {
				return ErrOrderRegByThatUser
			} else {
				return ErrOrderRegByOtherUser
			}
		}
	}

	order := MockOrderStatus{
		Number:     orderNumber,
		Status:     "NEW",
		UploadedAt: time.Now(),
		UserId:     user.GetId(),
	}
	m.OrderStatus = append(m.OrderStatus, order)
	return nil
}

func (m *Mock) AddWithdrawn(orderNumber string, amount int64, user User) error {
	curBalance, err := m.GetBalance(user)
	if err != nil {
		return err
	}

	if curBalance < float64(amount) {
		return ErrBalanceExceeded
	}

	w := MockWithdrawn{
		OrderId:     orderNumber,
		Amount:      amount,
		ProcessedAt: time.Now(),
		UserId:      user.GetId(),
	}
	m.Withdrawn = append(m.Withdrawn, w)
	newBalance := curBalance - float64(amount)
	err = m.UpdateBalance(newBalance, user)
	if err != nil {
		return err
	}

	return nil
}

func (m *Mock) ResetData() {
	m.Users = MockUserData
	m.OrderStatus = MockOrderStatusData
	m.Withdrawn = MockWithdrawnData
	m.Balance = MockUserBalanceData
}

func (m *Mock) GetOrdersWithTemporaryStatus() ([]MockOrderStatus, error) {
	result := make([]MockOrderStatus, 0)
	for _, oS := range m.OrderStatus {
		if oS.Status == "NEW" || oS.Status == "PROCESSING" {
			result = append(result, oS)
		}
	}
	return result, nil
}

func (m *Mock) UpdateOrderStatuses(orderStatusList []MockOrderStatus) error {
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

func (m *Mock) GetAllOrderStatusList() ([]MockOrderStatus, error) {
	return m.OrderStatus, nil
}
