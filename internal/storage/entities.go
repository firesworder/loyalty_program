package storage

import "time"

type SQLUser struct {
	Id       int64
	Login    string
	Password string
}

func (u *SQLUser) GetToken() string {
	return ""
}

func (u *SQLUser) GetLogin() string {
	return u.Login
}

func (u *SQLUser) GetPassword() string {
	return u.Password
}

func (u *SQLUser) GetId() int64 {
	return u.Id
}

type SQLOrderStatus struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Amount     int64     `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
	UserId     int64     `json:"-"`
}

type SQLWithdrawn struct {
	OrderId     string    `json:"order"`
	Amount      int64     `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
	UserId      int64     `json:"-"`
}
