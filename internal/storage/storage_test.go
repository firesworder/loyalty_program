package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"
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
		Number:     "352346287613",
		Status:     "INVALID",
		UploadedAt: time.Date(2023, 03, 12, 12, 0, 0, 0, time.Local),
	},
}

func clearTables(t *testing.T, db *sql.DB) {
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
				err = db.Connection.PingContext(context.Background())
				assert.NoError(t, err)

				err = db.Connection.Close()
				require.NoError(t, err)
			}
		})
	}
}

func TestSQLStorage_AddUser(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// очищаю таблицы перед добавлением новых тестовых данных и по итогам прогона тестов
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	// подготовка тестовых данных
	var uID int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) returning id",
		"postgres", "postgres").Scan(&uID)
	require.NoError(t, err)
	_, err = db.Connection.ExecContext(ctx,
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)",
		0, 0, uID)
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
			args:    args{login: "postgres", password: "password"},
			wantErr: ErrLoginExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = db.AddUser(ctx, tt.login, tt.password)
			assert.ErrorIs(t, err, tt.wantErr)

			if tt.wantErr == nil {
				// проверяю, что пользователь появился в таблице пользователей
				var newUserID int64
				err = db.Connection.QueryRowContext(ctx,
					"SELECT id FROM users WHERE login = $1 AND password = $2 LIMIT 1",
					tt.login, tt.password).Scan(&newUserID)
				require.NoError(t, err)

				// проверяю, что пользователю был добавлен баланс в таблице балансов
				var newUserBalanceId int64
				err = db.Connection.QueryRowContext(ctx,
					"SELECT id FROM balance WHERE user_id = $1",
					newUserID).Scan(&newUserBalanceId)
				require.NoError(t, err)
			}
		})
	}
}

func TestSQLStorage_GetBalance(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// чищу таблицы до и после теста
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	// вставка демо данных
	var userID int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID)
	require.NoError(t, err)
	_, err = db.Connection.ExecContext(ctx,
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
			name: "Test 2. User balance not found",
			user: User{
				ID:       15,
				Login:    "someUser",
				Password: "someUser",
			},
			wantBalance: nil,
			wantErr:     fmt.Errorf("user balance was not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, err := db.GetBalance(ctx, tt.user)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantBalance, balance)
		})
	}
}

func TestSQLStorage_GetOrderStatusList(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// вставка демо данных
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = db.Connection.ExecContext(ctx,
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
		require.NoError(t, err)
	}
	require.NoError(t, err)

	tests := []struct {
		name    string
		user    User
		wantOS  []OrderStatus
		wantErr error
	}{
		{
			name: "Test 1. User has registered orders",
			user: User{
				ID:       userID1,
				Login:    "demoU",
				Password: "demoU",
			},
			wantOS:  []OrderStatus{demo[0], demo[2]},
			wantErr: nil,
		},
		{
			name: "Test 2. User has not registered orders",
			user: User{
				ID:       15,
				Login:    "someUser",
				Password: "someUser",
			},
			wantOS:  []OrderStatus{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOS, err := db.GetOrderStatusList(ctx, tt.user)
			assert.Equal(t, tt.wantOS, gotOS)
			assert.Equal(t, err, tt.wantErr)
		})
	}
}

func TestSQLStorage_AddOrder(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// вставка тестовых данных
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = db.Connection.ExecContext(context.Background(),
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
		require.NoError(t, err)
	}
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
}

func TestSQLStorage_GetWithdrawnList(t *testing.T) {
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// вставка тестовых данных
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(context.Background(),
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

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
			UserID:      userID1,
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

	tests := []struct {
		name      string
		user      User
		wantWList []Withdrawn
		wantErr   error
	}{
		{
			name: "Test 1. Correct request",
			user: User{
				ID:       userID1,
				Login:    "demoU",
				Password: "demoU",
			},
			wantWList: []Withdrawn{demoWithdrawn[0], demoWithdrawn[1], demoWithdrawn[2]},
			wantErr:   nil,
		},
		{
			name: "Test 2. User has no withdrawn",
			user: User{
				ID:       userID2,
				Login:    "user2",
				Password: "pw2",
			},
			wantWList: []Withdrawn{},
			wantErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWList, err := db.GetWithdrawnList(context.Background(), tt.user)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantWList, gotWList)
		})
	}
}

func TestSQLStorage_AddWithdrawn(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// чистка таблиц, до и после теста
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	// вставка тестовых данных
	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	_, err = db.Connection.ExecContext(ctx,
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)", 1000, 800, userID1)
	require.NoError(t, err)
	_, err = db.Connection.ExecContext(ctx,
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)", 500, 800, userID2)
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
					ID:       userID1,
					Login:    "demoU",
					Password: "demoU",
				},
			},
			wantBalance: &Balance{BalanceAmount: 800, WithdrawnAmount: 1000, UserID: userID1},
			wantErr:     nil,
		},
		{
			name: "Test 2. Incorrect request, balance exceeded",
			args: args{
				orderNumber: "9359943520",
				amount:      1500,
				user: User{
					ID:       userID2,
					Login:    "user2",
					Password: "pw2",
				},
			},
			wantBalance: &Balance{BalanceAmount: 500, WithdrawnAmount: 800, UserID: userID2},
			wantErr:     ErrBalanceExceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = db.AddWithdrawn(ctx, tt.orderNumber, tt.amount, tt.user)
			assert.ErrorIs(t, err, tt.wantErr)

			var balance Balance
			err = db.Connection.QueryRowContext(ctx,
				"SELECT balance, withdrawn, user_id FROM balance WHERE user_id = $1",
				tt.user.ID).Scan(&balance.BalanceAmount, &balance.WithdrawnAmount, &balance.UserID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBalance, &balance)
		})
	}
}

func TestSQLStorage_GetOrdersWithTemporaryStatus(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// вставка тестовых данных
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = db.Connection.ExecContext(ctx,
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
		require.NoError(t, err)
	}

	// тест
	oSList, err := db.GetOrdersWithTemporaryStatus(ctx)
	assert.Equal(t, []OrderStatus{demoOrderStatuses[0]}, oSList)
	assert.NoError(t, err)
}

func TestSQLStorage_GetUser(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// вставка тестовых данных
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	var userID1 int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id",
		"demoU", "demoU").Scan(&userID1)
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
			gotUser, err := db.GetUser(ctx, tt.login, tt.password)
			assert.Equal(t, tt.wantUser, gotUser)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestSQLStorage_UpdateOrderStatuses(t *testing.T) {
	ctx := context.Background()
	db, err := NewSQLStorage(devDSN)
	if err != nil {
		log.Println(err)
		t.Skipf("dev db is not available. skipping")
	}
	defer db.Connection.Close()

	// вставка тестовых данных
	clearTables(t, db.Connection)
	defer clearTables(t, db.Connection)

	var userID1, userID2 int64
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "demoU", "demoU").Scan(&userID1)
	require.NoError(t, err)
	err = db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id", "user2", "pw2").Scan(&userID2)
	require.NoError(t, err)

	_, err = db.Connection.ExecContext(ctx,
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)",
		1000, 1000, userID1)
	require.NoError(t, err)
	_, err = db.Connection.ExecContext(ctx,
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)",
		1000, 1000, userID2)
	require.NoError(t, err)

	demo := demoOrderStatuses
	demo[0].UserID, demo[2].UserID = userID1, userID1
	demo[1].UserID, demo[3].UserID = userID2, userID2
	for _, oS := range demo {
		_, err = db.Connection.ExecContext(ctx,
			"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
			oS.Number, oS.Status, oS.Amount, oS.UploadedAt, oS.UserID)
		require.NoError(t, err)
	}

	updatedOS := []OrderStatus{demo[0]}
	updatedOS[0].Status = "PROCESSED"
	updatedOS[0].Amount = 300

	wantOS := []OrderStatus{
		demo[1],
		demo[2],
		demo[3],
		updatedOS[0],
	}

	// запускаю тестируемую функцию
	err = db.UpdateOrderStatuses(ctx, updatedOS)
	require.NoError(t, err)

	// беру текущее состояние таблицы orders
	curOS := make([]OrderStatus, 0)
	rows, err := db.Connection.QueryContext(context.Background(),
		`SELECT order_id, status, amount, uploaded_at, user_id FROM orders`)
	require.NoError(t, err)
	defer rows.Close()
	var oS OrderStatus
	for rows.Next() {
		oS = OrderStatus{}
		err = rows.Scan(&oS.Number, &oS.Status, &oS.Amount, &oS.UploadedAt, &oS.UserID)
		require.NoError(t, err)

		curOS = append(curOS, oS)
	}
	err = rows.Err()
	require.NoError(t, err)

	// проверяю наличие изменений в таблице заказов
	assert.Equal(t, wantOS, curOS)

	// беру текущее состояние таблицы балансов
	balances := map[int64]Balance{}
	rows, err = db.Connection.QueryContext(ctx,
		`SELECT balance, withdrawn, user_id FROM balance`)
	require.NoError(t, err)
	defer rows.Close()
	var b Balance
	for rows.Next() {
		b = Balance{}
		err = rows.Scan(&b.BalanceAmount, &b.WithdrawnAmount, &b.UserID)
		require.NoError(t, err)

		balances[b.UserID] = b
	}
	err = rows.Err()
	require.NoError(t, err)

	// проверяю изменения балансов
	expectedBalances := map[int64]Balance{
		userID1: {
			UserID:          userID1,
			BalanceAmount:   1300,
			WithdrawnAmount: 1000,
		},
		userID2: {
			UserID:          userID2,
			BalanceAmount:   1000,
			WithdrawnAmount: 1000,
		},
	}
	assert.Equal(t, expectedBalances, balances)
}
