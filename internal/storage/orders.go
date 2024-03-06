package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (store SQLStore) CreateNewOrder(ctx context.Context, data OrderData) error {
	data.State = "NEW"
	dataLogin, ok := ctx.Value(UserLoginCtxKey).(string)
	if !ok {
		return ErrNoLogin
	}
	tx, err := store.DB.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = store.DB.Exec(ctx, `INSERT INTO orders 
	(order_number, accrual_points, state, customer, withdrawal, created) 
	values ($1, $2, $3, $4, $5, $6);`,
		data.OrderNumber, data.Accrual, data.State, dataLogin, data.Withdrawal, data.Date)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}
	return nil
}

func (store SQLStore) GetCustomerOrders(ctx context.Context) ([]OrderResponse, error) {
	dataLogin, ok := ctx.Value(UserLoginCtxKey).(string)

	if !ok {
		return nil, ErrNoLogin
	}
	result := []OrderResponse{}
	for {
		select {
		case <-ctx.Done():
			return result, errTimeout
		default:
			query := fmt.Sprintf(`SELECT order_number, accrual_points, state, created 
	FROM orders 
	WHERE customer = '%s' 
	ORDER BY id DESC`, dataLogin)

			rows, err := store.DB.Query(ctx, query)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			for rows.Next() {
				var order OrderResponse
				if err := rows.Scan(&order.OrderNumber, &order.Accrual, &order.State, &order.Date); err != nil {
					return result, err
				}
				order.Accrual = order.Accrual / 100
				result = append(result, order)
			}
			if err = rows.Err(); err != nil {
				return result, err
			}
			return result, nil
		}
	}

}

func (store SQLStore) CheckIfOrderExists(ctx context.Context, data OrderData) (bool, bool, error) {
	var query string
	dataLogin, ok := ctx.Value(UserLoginCtxKey).(string)
	if !ok {
		return false, false, ErrNoLogin
	}
	query = fmt.Sprintf(`SELECT order_number, customer 
	FROM orders 
	WHERE order_number = %d`, data.OrderNumber)
	var number uint64
	var login string
	err := store.DB.QueryRow(ctx, query).Scan(&number, &login)
	if errors.Is(err, pgx.ErrNoRows) {
		//No order
		return true, false, err
	}
	// Order exists for another user
	if login != dataLogin {
		return false, true, err
	}
	// order already exists for current user
	return false, false, err
}

func (store SQLStore) GetUnfinishedOrders() ([]uint64, error) {
	sqlQuery := "SELECT order_number FROM orders WHERE state IN ('NEW', 'PROCESSING')"
	ctx := context.Background()
	var result []uint64
	rows, err := store.DB.Query(ctx, sqlQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var order uint64
		if err := rows.Scan(&order); err != nil {
			return result, err
		}
		result = append(result, order)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil

}

func (store SQLStore) UpdateOrder(ctx context.Context, data OrderData) error {
	sql := `
	UPDATE orders 
	SET accrual_points = $1, state = $2 
	WHERE order_number = $3;
`
	for {
		select {
		case <-ctx.Done():
			return errTimeout
		default:
			tx, err := store.DB.Begin(ctx)
			if err != nil {
				return err
			}

			_, err = store.DB.Exec(ctx, sql, data.Accrual, data.State, data.OrderNumber)
			if err != nil {
				tx.Rollback(ctx)
				return err
			}
			err = tx.Commit(ctx)
			if err != nil {
				tx.Rollback(ctx)
				return err
			}
			return err
		}
	}

}
