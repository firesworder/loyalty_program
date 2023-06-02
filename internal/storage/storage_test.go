package storage

import (
	"context"
	"database/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const devDSN = "postgresql://postgres:admin@localhost:5432/loyalty_program"

var demoOrderStatuses = []OrderStatus{
	{
		Number:     "9359943520",
		Status:     "NEW",
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
	},
	{
		Number:     "328257446760",
		Status:     "PROCESSED",
		Amount:     900,
		UploadedAt: time.Date(2023, 03, 11, 12, 0, 0, 0, time.Local),
	},
	{
		Number:     "000971335161",
		Status:     "PROCESSED",
		Amount:     400,
		UploadedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
	},
	{
		Number:     "328257446760",
		Status:     "INVALID",
		UploadedAt: time.Date(2023, 03, 12, 12, 0, 0, 0, time.Local),
	},
}

// todo: добавить user UID и убрать привязку с ID
// todo: проверять уникальность записей в балансах(для одного пользователя - одна строка)

func undoTestChanges(t *testing.T, db *sql.DB) {
	var err error
	_, err = db.ExecContext(context.Background(), "DELETE FROM withdrawn")
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), "DELETE FROM orders")
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), "DELETE FROM balance")
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), "DELETE FROM users")
	require.NoError(t, err)
}

func TestNewSQLStorage(t *testing.T) {
	tests := []struct {
		name    string
		DSN     string
		wantErr bool
	}{
		{
			name:    "Test 1. Correct DSN.",
			DSN:     "postgresql://postgres:admin@localhost:5432/loyalty_program",
			wantErr: false,
		},
		{
			name:    "Test 2. Incorrect DSN.",
			DSN:     "postgresql://postgres:admin@localhost:5432/program",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewSQLStorage(tt.DSN)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				err = db.Connection.Ping()
				assert.NoError(t, err)
			}
		})
	}
}

func TestSQLStorage_AddUser(t *testing.T) {
	db, err := NewSQLStorage(devDSN)
	require.NoError(t, err)

	type args struct {
		login, password string
	}

	tests := []struct {
		name string
		args
		wantErr error
	}{
		{
			name:    "Test 1. Correct new user",
			args:    args{login: "demo1", password: "demo1"},
			wantErr: nil,
		},
		// todo: тесты должно не зависеть друг от друга
		{
			name:    "Test 2. User with that login already exist",
			args:    args{login: "demo1", password: "password"},
			wantErr: ErrLoginExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = db.AddUser(tt.login, tt.password)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_GetBalance(t *testing.T) {
	db, err := NewSQLStorage(devDSN)
	require.NoError(t, err)

	// вставка демо данных
	undoTestChanges(t, db.Connection)

	var userId int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userId)
	require.NoError(t, err)
	_, err = db.Connection.ExecContext(context.Background(),
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)",
		250, 130, userId)
	require.NoError(t, err)

	tests := []struct {
		name        string
		user        User
		wantBalance float64
		wantErr     error
	}{
		{
			name: "Test 1. Correct getBalancer",
			user: User{
				ID:       userId,
				Login:    "demoU",
				Password: "demoU",
			},
			wantBalance: 250,
			wantErr:     nil,
		},
		{
			name: "Test 2. User not found",
			user: User{
				ID:       15,
				Login:    "someUser",
				Password: "someUser",
			},
			wantBalance: 0,
			wantErr:     sql.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, err := db.GetBalance(tt.user)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantBalance, balance)
		})
	}
	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_GetOrderStatusList(t *testing.T) {
	db, err := NewSQLStorage(devDSN)
	require.NoError(t, err)

	// вставка демо данных
	undoTestChanges(t, db.Connection)

	var userId1, userId2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userId1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userId2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demo := demoOrderStatuses
	demo[0].UserId, demo[2].UserId = userId1, userId1
	demo[1].UserId, demo[3].UserId = userId2, userId2
	for _, oS := range demo {
		_, err = tx.ExecContext(context.Background(),
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserId)
		require.NoError(t, err)
	}
	err = tx.Commit()
	require.NoError(t, err)

	tests := []struct {
		name   string
		user   User
		wantOS []OrderStatus
		//wantErr     error
	}{
		{
			name: "Test 1. User has registered orders",
			user: User{
				ID:       userId1,
				Login:    "demoU",
				Password: "demoU",
			},
			wantOS: []OrderStatus{demo[0], demo[2]},
		},
		{
			name: "Test 2. User has not registered orders",
			user: User{
				ID:       15,
				Login:    "someUser",
				Password: "someUser",
			},
			wantOS: []OrderStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOS := db.GetOrderStatusList(tt.user)
			assert.Equal(t, tt.wantOS, gotOS)
		})
	}
	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_AddOrder(t *testing.T) {
	db, err := NewSQLStorage(devDSN)
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userId1, userId2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userId1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userId2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demo := demoOrderStatuses
	demo[0].UserId, demo[2].UserId = userId1, userId1
	demo[1].UserId, demo[3].UserId = userId2, userId2
	for _, oS := range demo {
		_, err = tx.ExecContext(context.Background(),
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserId)
		require.NoError(t, err)
	}
	err = tx.Commit()
	require.NoError(t, err)

	type args struct {
		orderNumber string
		user        User
	}
	tests := []struct {
		name string
		args
		wantErr error
	}{
		{
			name: "Test 1. Correct add order",
			args: args{
				orderNumber: "624605372751",
				user: User{
					ID:       userId1,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantErr: nil,
		},
		{
			name: "Test 2. Incorrect request, order registered by the same user",
			args: args{
				orderNumber: "9359943520",
				user: User{
					ID:       userId1,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantErr: ErrOrderRegByThatUser,
		},
		{
			name: "Test 3. Incorrect request, order registered by other user",
			args: args{
				orderNumber: "328257446760",
				user: User{
					ID:       userId1,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantErr: ErrOrderRegByOtherUser,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = db.AddOrder(tt.orderNumber, tt.args.user)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_GetWithdrawnList(t *testing.T) {
	db, err := NewSQLStorage(devDSN)
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userId1, userId2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userId1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userId2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demoWithdrawn := []Withdrawn{
		{
			OrderId:     "624605372751",
			Amount:      300,
			ProcessedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
			UserId:      userId1,
		},
		{
			OrderId:     "000971335161",
			Amount:      50,
			ProcessedAt: time.Date(2023, 02, 10, 12, 0, 0, 0, time.Local),
			UserId:      userId2,
		},
		{
			OrderId:     "9359943520",
			Amount:      125,
			ProcessedAt: time.Date(2023, 01, 10, 12, 0, 0, 0, time.Local),
			UserId:      userId1,
		},
	}

	for _, w := range demoWithdrawn {
		_, err = db.Connection.ExecContext(context.Background(),
			"INSERT INTO withdrawn(order_id, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4)",
			w.OrderId, w.Amount, w.ProcessedAt, w.UserId)
		require.NoError(t, err)
	}
	err = tx.Commit()
	require.NoError(t, err)

	tests := []struct {
		name      string
		user      User
		wantWList []Withdrawn
		// todo: добавить ошибку
		//wantErr error
	}{
		{
			name: "Test 1. Correct request",
			user: User{
				ID:       userId1,
				Login:    "demoU",
				Password: "demoU",
			},
			wantWList: []Withdrawn{demoWithdrawn[0], demoWithdrawn[2]},
		},
		{
			name: "Test 2. User has no withdrawn",
			user: User{
				ID:       15,
				Login:    "someUser",
				Password: "someUser",
			},
			wantWList: []Withdrawn{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWList := db.GetWithdrawnList(tt.user)
			assert.Equal(t, tt.wantWList, gotWList)
		})
	}

	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_AddWithdrawn(t *testing.T) {
	db, err := NewSQLStorage(devDSN)
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userId int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userId)
	require.NoError(t, err)

	_, err = db.Connection.ExecContext(context.Background(),
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)", 1000, 800, userId)
	require.NoError(t, err)

	type args struct {
		orderNumber string
		amount      int64
		user        User
	}
	tests := []struct {
		name string
		args
		// todo: нужно проверять и withdrawn в балансе
		wantBalance int64
		wantErr     error
	}{
		{
			name: "Test 1. Correct request",
			args: args{
				orderNumber: "328257446760",
				amount:      200,
				user: User{
					ID:       userId,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantBalance: 800,
			wantErr:     nil,
		},
		{
			name: "Test 2. Incorrect request, balance exceeded",
			args: args{
				orderNumber: "9359943520",
				amount:      1500,
				user: User{
					ID:       userId,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantBalance: 800,
			wantErr:     ErrBalanceExceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = db.AddWithdrawn(tt.orderNumber, tt.amount, tt.user)
			assert.ErrorIs(t, err, tt.wantErr)

			balance, err := db.GetBalance(tt.user)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBalance, int64(balance))
		})
	}
}
