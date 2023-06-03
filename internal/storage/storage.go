package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"log"
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

func (db *SQLStorage) GetUser(ctx context.Context, login, password string) (*User, error) {
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

func (db *SQLStorage) GetBalance(ctx context.Context, user User) (*Balance, error) {
	var b, w, uid int64
	err := db.Connection.QueryRowContext(ctx,
		"SELECT balance, withdrawn, user_id FROM balance WHERE user_id = $1 LIMIT 1", user.ID,
	).Scan(&b, &w, &uid)
	if err != nil {
		return nil, err
	}
	return &Balance{UserId: uid, BalanceAmount: b, WithdrawnAmount: w}, nil
}

func (db *SQLStorage) GetOrderStatusList(ctx context.Context, user User) []OrderStatus {
	result := make([]OrderStatus, 0)
	rows, err := db.Connection.QueryContext(ctx,
		`SELECT order_id, status, amount, uploaded_at, user_id FROM orders WHERE user_id = $1`, user.ID)

	var oS OrderStatus
	for rows.Next() {
		oS = OrderStatus{}
		err = rows.Scan(&oS.Number, &oS.Status, &oS.Amount, &oS.UploadedAt, &oS.UserId)
		if err != nil {
			return nil
		}

		result = append(result, oS)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
		return nil
	}
	return result
}

func (db *SQLStorage) AddOrder(ctx context.Context, orderNumber string, user User) error {
	// проверка существования заказа в orders
	exOrderID, exUserID := "", int64(0)
	err := db.Connection.QueryRowContext(ctx,
		"SELECT order_id, user_id FROM orders WHERE order_id = $1", orderNumber).Scan(&exOrderID, &exUserID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if exOrderID != "" {
		if exUserID == user.ID {
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

func (db *SQLStorage) AddWithdrawn(ctx context.Context, orderNumber string, amount int64, user User) error {
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

func (db *SQLStorage) GetWithdrawnList(ctx context.Context, user User) []Withdrawn {
	result := make([]Withdrawn, 0)
	rows, err := db.Connection.QueryContext(ctx,
		`SELECT order_id, amount, uploaded_at, user_id FROM withdrawn WHERE user_id = $1`,
		user.ID)

	var w Withdrawn
	for rows.Next() {
		w = Withdrawn{}
		err = rows.Scan(&w.OrderID, &w.Amount, &w.ProcessedAt, &w.UserId)
		if err != nil {
			return nil
		}

		result = append(result, w)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
		return nil
	}
	return result
}

func (db *SQLStorage) GetOrdersWithTemporaryStatus(ctx context.Context) ([]OrderStatus, error) {
	result := make([]OrderStatus, 0)
	rows, err := db.Connection.QueryContext(ctx,
		`SELECT order_id, status, amount, uploaded_at, user_id FROM orders WHERE status IN ($1, $2)`,
		"NEW", "PROCESSING")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		oS := OrderStatus{}
		err := rows.Scan(&oS.Number, &oS.Status, &oS.Amount, &oS.UploadedAt, &oS.UserId)
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

	for _, oS := range orderStatusList {
		_, err = tx.ExecContext(ctx,
			"UPDATE orders SET status = $1, amount = $2 WHERE order_id = $3",
			oS.Status, oS.Amount, oS.Number)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *SQLStorage) UpdateBalance(ctx context.Context, newBalance Balance) error {
	result, err := db.Connection.ExecContext(ctx,
		"UPDATE balance SET balance = $1, withdrawn = $2 WHERE user_id = $3",
		newBalance.BalanceAmount, newBalance.WithdrawnAmount, newBalance.UserId)
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
	return nil
}
