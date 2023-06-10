package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"time"
)

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

func (db *SQLStorage) AddUser(ctx context.Context, login, password string) (*User, error) {
	var id int64
	tx, err := db.Connection.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Добавляем пользователя в соотв. таблицу
	err = tx.QueryRowContext(ctx,
		"INSERT INTO users(login, password) VALUES ($1, $2) RETURNING id",
		login, password,
	).Scan(&id)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return nil, ErrLoginExist
		}
		return nil, err
	}

	// Добавляем баланс пользователя
	_, err = tx.ExecContext(ctx,
		"INSERT INTO balance(balance, withdrawn, user_id) VALUES ($1, $2, $3)",
		0, 0, id)
	if err != nil {
		return nil, err
	}

	return &User{ID: id, Login: login, Password: password}, tx.Commit()
}

func (db *SQLStorage) GetUser(ctx context.Context, login, password string) (*User, error) {
	u := User{}
	err := db.Connection.QueryRowContext(ctx,
		"SELECT id, login, password FROM users WHERE login = $1 AND password = $2 LIMIT 1",
		login, password).Scan(&u.ID, &u.Login, &u.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAuthDataIncorrect
		}
		return nil, err
	}
	return &u, nil
}

func (db *SQLStorage) GetBalance(ctx context.Context, user User) (*Balance, error) {
	var uid int64
	var b, w float64
	err := db.Connection.QueryRowContext(ctx,
		"SELECT balance, withdrawn, user_id FROM balance WHERE user_id = $1 LIMIT 1",
		user.ID).Scan(&b, &w, &uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user balance was not found")
		}
		return nil, err
	}
	return &Balance{UserID: uid, BalanceAmount: b, WithdrawnAmount: w}, nil
}

func (db *SQLStorage) GetOrderStatusList(ctx context.Context, user User) ([]OrderStatus, error) {
	result := make([]OrderStatus, 0)
	rows, err := db.Connection.QueryContext(ctx,
		`SELECT order_id, status, amount, uploaded_at, user_id FROM orders WHERE user_id = $1`, user.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var oS OrderStatus
	for rows.Next() {
		oS = OrderStatus{}
		err = rows.Scan(&oS.Number, &oS.Status, &oS.Amount, &oS.UploadedAt, &oS.UserID)
		if err != nil {
			return nil, err
		}

		result = append(result, oS)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (db *SQLStorage) AddOrder(ctx context.Context, orderNumber string, user User) error {
	// Попытка вставить заказ в соотв-ую таблицу
	_, err := db.Connection.ExecContext(ctx,
		"INSERT INTO orders(order_id, status, amount, uploaded_at, user_id) VALUES ($1, $2, $3, $4, $5)",
		orderNumber, "NEW", 0, time.Now(), user.ID)
	// Если заказ добавился без ошибки - прекратить функцию, иначе продолжить
	if err == nil {
		return nil
	} else if pgErr, ok := err.(*pgconn.PgError);
	// если ошибка не относится к нар-ию уникальности номера заказа - вернуть ошибку
	!ok || !(pgErr.Code == "23505" && pgErr.ConstraintName == "orders_order_id_key") {
		return err
	}

	// Уточнение ошибки
	var userRegOrder int64
	err = db.Connection.QueryRowContext(ctx,
		"SELECT user_id FROM orders WHERE order_id = $1", orderNumber).Scan(&userRegOrder)
	if err != nil {
		return err
	}

	if userRegOrder == user.ID {
		return ErrOrderRegByThatUser
	}
	return ErrOrderRegByOtherUser
}

func (db *SQLStorage) AddWithdrawn(ctx context.Context, orderNumber string, amount float64, user User) error {
	// попытка выполнить списание с баланса, транзакцией
	tx, err := db.Connection.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	// изменение баланса(значений баланса бонусов и кол-ва списанных баллов)
	result, err := tx.ExecContext(ctx,
		"UPDATE balance SET balance = balance - $1, withdrawn = withdrawn + $2 WHERE user_id = $3",
		amount, amount, user.ID)
	if err != nil {
		pgErr, ok := err.(*pgconn.PgError)
		if ok && (pgErr.Code == "23514" && pgErr.ConstraintName == "balance_balance_check") {
			return ErrBalanceExceeded
		}
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
	_, err = tx.ExecContext(ctx,
		"INSERT INTO withdrawn(order_id, amount, uploaded_at, user_id) VALUES($1, $2, $3, $4)",
		orderNumber, amount, time.Now(), user.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (db *SQLStorage) GetWithdrawnList(ctx context.Context, user User) ([]Withdrawn, error) {
	result := make([]Withdrawn, 0)
	rows, err := db.Connection.QueryContext(ctx,
		`SELECT order_id, amount, uploaded_at, user_id FROM withdrawn WHERE user_id = $1`,
		user.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var w Withdrawn
	for rows.Next() {
		w = Withdrawn{}
		err = rows.Scan(&w.OrderID, &w.Amount, &w.ProcessedAt, &w.UserID)
		if err != nil {
			return nil, err
		}

		result = append(result, w)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (db *SQLStorage) GetOrdersWithTemporaryStatus(ctx context.Context) ([]OrderStatus, error) {
	result := make([]OrderStatus, 0)
	rows, err := db.Connection.QueryContext(ctx,
		`SELECT order_id, status, amount, uploaded_at, user_id FROM orders WHERE status IN ($1, $2)`,
		"NEW", "PROCESSING")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		oS := OrderStatus{}
		err := rows.Scan(&oS.Number, &oS.Status, &oS.Amount, &oS.UploadedAt, &oS.UserID)
		if err != nil {
			return nil, err
		}
		result = append(result, oS)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (db *SQLStorage) UpdateOrderStatuses(ctx context.Context, orderStatusList []OrderStatus) error {
	tx, err := db.Connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	userBalanceUpdates := map[int64]float64{}
	for _, oS := range orderStatusList {
		if oS.Status == "PROCESSED" && oS.Amount != 0 {
			userBalanceUpdates[oS.UserID] += oS.Amount
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE orders SET status = $1, amount = $2 WHERE order_id = $3",
			oS.Status, oS.Amount, oS.Number)
		if err != nil {
			return err
		}
	}

	for userID, bUpdates := range userBalanceUpdates {
		_, err = tx.ExecContext(ctx,
			"UPDATE balance SET balance = balance + $1 WHERE user_id = $2",
			bUpdates, userID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
