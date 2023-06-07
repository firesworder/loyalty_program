package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const devDSN = "postgresql://postgres:admin@localhost:5432/loyalty_program"

var isDevDBAvailable = true

func init() {
	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	if err != nil {
		isDevDBAvailable = false
	}
}

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
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

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
			defer db.Connection.Close()
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				err = db.Connection.PingContext(context.Background())
				assert.NoError(t, err)
			}
		})
	}
}

func TestSQLStorage_AddUser(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
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
		{
			name:    "Test 2. User with that login already exist",
			args:    args{login: "demo1", password: "password"},
			wantErr: ErrLoginExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = db.AddUser(context.Background(), tt.login, tt.password)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_GetBalance(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка демо данных
	undoTestChanges(t, db.Connection)

	var userID int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID)
	require.NoError(t, err)
	_, err = db.Connection.ExecContext(context.Background(),
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)",
		250, 130, userID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		user        User
		wantBalance *Balance
		wantErr     error
	}{
		{
			name: "Test 1. Correct getBalance.",
			user: User{
				ID:       userID,
				Login:    "demoU",
				Password: "demoU",
			},
			wantBalance: &Balance{UserID: userID, BalanceAmount: 250, WithdrawnAmount: 130},
			wantErr:     nil,
		},
		{
			name: "Test 2. User not found",
			user: User{
				ID:       15,
				Login:    "someUser",
				Password: "someUser",
			},
			wantBalance: nil,
			wantErr:     sql.ErrNoRows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, err := db.GetBalance(context.Background(), tt.user)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantBalance, balance)
		})
	}
	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_GetOrderStatusList(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка демо данных
	undoTestChanges(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = tx.ExecContext(context.Background(),
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
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
				ID:       userID1,
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
			gotOS := db.GetOrderStatusList(context.Background(), tt.user)
			assert.Equal(t, tt.wantOS, gotOS)
		})
	}
	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_AddOrder(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = tx.ExecContext(context.Background(),
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
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
					ID:       userID1,
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
					ID:       userID1,
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
					ID:       userID1,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantErr: ErrOrderRegByOtherUser,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = db.AddOrder(context.Background(), tt.orderNumber, tt.args.user)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_GetWithdrawnList(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demoWithdrawn := []Withdrawn{
		{
			OrderID:     "624605372751",
			Amount:      300,
			ProcessedAt: time.Date(2023, 03, 10, 12, 0, 0, 0, time.Local),
			UserID:      userID1,
		},
		{
			OrderID:     "000971335161",
			Amount:      50,
			ProcessedAt: time.Date(2023, 02, 10, 12, 0, 0, 0, time.Local),
			UserID:      userID2,
		},
		{
			OrderID:     "9359943520",
			Amount:      125,
			ProcessedAt: time.Date(2023, 01, 10, 12, 0, 0, 0, time.Local),
			UserID:      userID1,
		},
	}

	for _, w := range demoWithdrawn {
		_, err = db.Connection.ExecContext(context.Background(),
			"INSERT INTO withdrawn(order_id, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4)",
			w.OrderID, w.Amount, w.ProcessedAt, w.UserID)
		require.NoError(t, err)
	}
	err = tx.Commit()
	require.NoError(t, err)

	tests := []struct {
		name      string
		user      User
		wantWList []Withdrawn
		//wantErr error
	}{
		{
			name: "Test 1. Correct request",
			user: User{
				ID:       userID1,
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
			gotWList := db.GetWithdrawnList(context.Background(), tt.user)
			assert.Equal(t, tt.wantWList, gotWList)
		})
	}

	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_AddWithdrawn(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userID int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID)
	require.NoError(t, err)

	_, err = db.Connection.ExecContext(context.Background(),
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)", 1000, 800, userID)
	require.NoError(t, err)

	type args struct {
		orderNumber string
		amount      float64
		user        User
	}
	tests := []struct {
		name string
		args
		wantBalance *Balance
		wantErr     error
	}{
		{
			name: "Test 1. Correct request",
			args: args{
				orderNumber: "328257446760",
				amount:      200,
				user: User{
					ID:       userID,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantBalance: &Balance{BalanceAmount: 800, WithdrawnAmount: 1000, UserID: userID},
			wantErr:     nil,
		},
		{
			name: "Test 2. Incorrect request, balance exceeded",
			args: args{
				orderNumber: "9359943520",
				amount:      1500,
				user: User{
					ID:       userID,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			// влияет тест сверху! если запускать отдельно - цифры будут отличаться(1000 и 800)
			wantBalance: &Balance{BalanceAmount: 800, WithdrawnAmount: 1000, UserID: userID},
			wantErr:     ErrBalanceExceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = db.AddWithdrawn(context.Background(), tt.orderNumber, tt.amount, tt.user)
			assert.ErrorIs(t, err, tt.wantErr)

			balance, err := db.GetBalance(context.Background(), tt.user)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBalance, balance)
		})
	}
}

func TestSQLStorage_GetOrdersWithTemporaryStatus(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = tx.ExecContext(context.Background(),
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
		require.NoError(t, err)
	}
	err = tx.Commit()
	require.NoError(t, err)

	tests := []struct {
		name    string
		wantOS  []OrderStatus
		wantErr error
	}{
		{
			name:    "Test 1. Correct add order",
			wantOS:  []OrderStatus{demoOrderStatuses[0]},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oSList, err := db.GetOrdersWithTemporaryStatus(context.Background())
			assert.Equal(t, tt.wantOS, oSList)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_GetUser(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userID1 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)

	type args struct {
		login, password string
	}

	tests := []struct {
		name string
		args
		wantUser *User
		wantErr  error
	}{
		{
			name: "Test 1. User exist",
			args: args{login: "demoU", password: "demoU"},
			wantUser: &User{
				ID:       userID1,
				Login:    "demoU",
				Password: "demoU",
			},
			wantErr: nil,
		},
		{
			name:     "Test 2. User not exist",
			args:     args{login: "someUser", password: "someUserPass"},
			wantUser: nil,
			wantErr:  ErrAuthDataIncorrect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUser, err := db.GetUser(context.Background(), tt.login, tt.password)
			assert.Equal(t, tt.wantUser, gotUser)
			assert.ErrorIs(t, err, tt.wantErr)

		})
	}

	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_UpdateBalance(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userID int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID)
	require.NoError(t, err)

	_, err = db.Connection.ExecContext(context.Background(),
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)",
		250, 130, userID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		argBalance  Balance
		wantBalance *Balance
		wantErr     error
	}{
		{
			name:        "Test 1. Correct update balance",
			argBalance:  Balance{UserID: userID, WithdrawnAmount: 150, BalanceAmount: 230},
			wantBalance: &Balance{UserID: userID, WithdrawnAmount: 150, BalanceAmount: 230},
			wantErr:     nil,
		},
		{
			name:        "Test 2. Incorrect update, user not found",
			argBalance:  Balance{UserID: 31333, WithdrawnAmount: 300, BalanceAmount: 500},
			wantBalance: &Balance{UserID: 0, BalanceAmount: 0, WithdrawnAmount: 0},
			wantErr:     fmt.Errorf("balance has not been changed, unknown error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = db.UpdateBalance(context.Background(), tt.argBalance)
			assert.Equal(t, err, tt.wantErr)

			// получаем текущие значения баланса из бд
			balance := &Balance{}
			err = db.Connection.QueryRowContext(context.Background(),
				"SELECT balance, withdrawn, user_id FROM balance WHERE user_id = $1",
				tt.argBalance.UserID).Scan(&balance.BalanceAmount, &balance.WithdrawnAmount, &balance.UserID)

			// проверяем наличие изменений
			assert.Equal(t, tt.wantBalance, balance)
		})
	}

	undoTestChanges(t, db.Connection)
}

func TestSQLStorage_UpdateOrderStatuses(t *testing.T) {
	if !isDevDBAvailable {
		t.Skipf("dev db is not available. skipping")
	}

	db, err := NewSQLStorage(devDSN)
	defer db.Connection.Close()
	require.NoError(t, err)

	// вставка тестовых данных
	undoTestChanges(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	// для пакетной вставки данных в дб
	tx, err := db.Connection.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = tx.ExecContext(context.Background(),
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
		require.NoError(t, err)
	}
	err = tx.Commit()
	require.NoError(t, err)

	updatedOS := []OrderStatus{demo[0]}
	updatedOS[0].Status = "PROCESSED"
	updatedOS[0].Amount = 300

	wantOS := []OrderStatus{
		demo[1],
		demo[2],
		demo[3],
		updatedOS[0],
	}

	// само тестирование
	err = db.UpdateOrderStatuses(context.Background(), updatedOS)
	require.NoError(t, err)

	// беру текущее состояние таблицы orders
	curOS := make([]OrderStatus, 0)
	rows, err := db.Connection.QueryContext(context.Background(),
		`SELECT order_id, status, amount, uploaded_at, user_id FROM orders`)
	require.NoError(t, err)
	var oS OrderStatus
	for rows.Next() {
		oS = OrderStatus{}
		err = rows.Scan(&oS.Number, &oS.Status, &oS.Amount, &oS.UploadedAt, &oS.UserID)
		require.NoError(t, err)

		curOS = append(curOS, oS)
	}
	err = rows.Err()
	require.NoError(t, err)

	assert.Equal(t, wantOS, curOS)

	undoTestChanges(t, db.Connection)
}
