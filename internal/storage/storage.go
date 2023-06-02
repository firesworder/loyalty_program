package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"time"
)

// todo: добавить везде контекст!!!

type SQLStorage struct {
	Connection *sql.DB
}

func NewSQLStorage(DSN string) (*SQLStorage, error) {
	// Этот метод вызывается при инициализации сервера, поэтому использую общий контекст
	ctx := context.Background()

	db := SQLStorage{}
	err := db.OpenDBConnection(DSN)
	if err != nil {
		return nil, err
	}
	err = db.CreateTablesIfNotExists(ctx)
	if err != nil {
		return nil, err
	}
	return &db, nil
}

func (db *SQLStorage) OpenDBConnection(DSN string) error {
	var err error
	db.Connection, err = sql.Open("pgx", DSN)
	if err != nil {
		return err
	}
	return nil
}

func (db *SQLStorage) CreateTablesIfNotExists(ctx context.Context) (err error) {
	_, err = db.Connection.ExecContext(ctx, createTablesSQL)
	if err != nil {
		return
	}
	return nil
}

func (db *SQLStorage) AddUser(login, password string) (*User, error) {
	ctx := context.Background()

	var id int64
	err := db.Connection.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id",
		login, password,
	).Scan(&id)
	if err != nil {
		pgErr := err.(*pgconn.PgError)
		if pgErr.Code == "23505" {
			return nil, ErrLoginExist
		}
		return nil, err
	}

	u := User{ID: id, Login: login, Password: password}
	return &u, nil
}

func (db *SQLStorage) GetUser(login, password string) (*User, error) {
	ctx := context.Background()

	u := User{}
	err := db.Connection.QueryRowContext(ctx,
		"SELECT id, login, password FROM users WHERE login = $1 AND password = $2",
		login, password).Scan(&u.ID, &u.Login, &u.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAuthDataIncorrect
		}
		return nil, err
	}
	return &u, nil
}

// todo: float переделать в int
func (db *SQLStorage) GetBalance(user User) (int64, error) {
	ctx := context.Background()
	var b int64
	err := db.Connection.QueryRowContext(ctx,
		"SELECT balance FROM balance WHERE user_id = $1 LIMIT 1", user.ID,
	).Scan(&b)
	if err != nil {
		return 0, err
	}
	return b, nil
}

func (db *SQLStorage) GetOrderStatusList(user User) []OrderStatus {
	result := make([]OrderStatus, 0)
	rows, err := db.Connection.QueryContext(context.Background(),
		`SELECT order_id, status, amount, uploaded_at, user_id FROM orders WHERE user_id = $1`, user.ID)

	var oS OrderStatus
	for rows.Next() {
		oS = OrderStatus{}
		err = rows.Scan(&oS.Number, &oS.Status, &oS.Amount, &oS.UploadedAt, &oS.UserId)
		// todo: тут должна быть ошибка
		if err != nil {
			return nil
		}

		result = append(result, oS)
	}
	return result
}

func (db *SQLStorage) AddOrder(orderNumber string, user User) error {
	ctx := context.Background()
	// проверка существования заказа в orders
	exOrderId, exUserId := "", int64(0)
	err := db.Connection.QueryRowContext(ctx,
		"SELECT order_id, user_id FROM orders WHERE order_id = $1", orderNumber).Scan(&exOrderId, &exUserId)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if exOrderId != "" {
		if exUserId == user.ID {
			return ErrOrderRegByThatUser
		} else {
			return ErrOrderRegByOtherUser
		}
	}

	// вставка
	result, err := db.Connection.ExecContext(ctx,
		"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
		orderNumber, "NEW", 0, time.Now(), user.ID)
	if err != nil {
		return err
	}
	_, err = result.RowsAffected()
	if err != nil {
		return err
	}
	return nil
}

func (db *SQLStorage) GetWithdrawn(user User) int64 {
	// todo: удалить метод
	return 0
}

func (db *SQLStorage) AddWithdrawn(orderNumber string, amount int64, user User) error {
	ctx := context.Background()
	// проверка баланса на возможность списания
	var curBalance, curWithdrawn int64
	err := db.Connection.QueryRowContext(ctx,
		"SELECT balance, withdrawn FROM balance WHERE user_id = $1", user.ID).Scan(&curBalance, &curWithdrawn)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no user balance data")
		}
		return err
	}

	// если текущий баланс меньше запрошенного списания
	if curBalance < amount {
		return ErrBalanceExceeded
	}

	// добавление запроса на списание
	tx, err := db.Connection.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()
	// изменение баланса оставшихся бонусов и списанных
	result, err := tx.ExecContext(ctx,
		"UPDATE balance SET balance = $1, withdrawn = $2 WHERE user_id = $3",
		curBalance-amount, curWithdrawn+amount, user.ID)
	if err != nil {
		return err
	}
	rAff, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rAff == 0 {
		return fmt.Errorf("balance has not been changed, unknown error")
	}

	// регистрация запроса на списание
	result, err = db.Connection.ExecContext(ctx,
		"INSERT INTO withdrawn(order_id, amount, uploaded_at, user_id) VALUES($1, $2, $3, $4)",
		orderNumber, amount, time.Now(), user.ID)
	if err != nil {
		return err
	}
	rAff, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rAff == 0 {
		return fmt.Errorf("withdraw was not complete, unknown error")
	}
	return tx.Commit()
}

func (db *SQLStorage) GetWithdrawnList(user User) []Withdrawn {
	ctx := context.Background()
	result := make([]Withdrawn, 0)
	rows, err := db.Connection.QueryContext(ctx,
		`SELECT order_id, amount, uploaded_at, user_id FROM withdrawn WHERE user_id = $1`,
		user.ID)

	var w Withdrawn
	for rows.Next() {
		w = Withdrawn{}
		// todo: orderId заменить на orderNumber
		err = rows.Scan(&w.OrderId, &w.Amount, &w.ProcessedAt, &w.UserId)
		// todo: тут должна быть ошибка
		if err != nil {
			return nil
		}

		result = append(result, w)
	}
	return result
}

func (db *SQLStorage) GetOrdersWithTemporaryStatus() ([]OrderStatus, error) {
	return nil, nil
}

func (db *SQLStorage) UpdateOrderStatuses(orderStatusList []OrderStatus) error {
	return nil
}

func (db *SQLStorage) GetAllOrderStatusList() ([]OrderStatus, error) {
	//TODO implement me
	panic("implement me")
}

func (db *SQLStorage) UpdateBalance(newAmount int64, user User) error {
	//TODO implement me
	panic("implement me")
}
