package storage

import (
	"context"
	"errors"
	"fmt"
)

func (store SQLStore) AddBalanceToUser(orderData OrderData) (bool, error) {
	ctx := context.Background()
	sqlQuery := fmt.Sprintf(`SELECT users.accrual_points, users.login 
	FROM users
	LEFT JOIN orders  
	ON users.login = orders.customer 
	WHERE orders.order_number = '%d'`, orderData.OrderNumber)
	var currentBalance int
	var login string

	err := store.DB.QueryRow(ctx, sqlQuery).Scan(&currentBalance, &login)
	if err != nil {
		return false, err
	}
	currentBalance += orderData.Accrual
	tx, err := store.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	sql := `UPDATE users SET accrual_points = $1 WHERE login = $2`
	_, err = store.DB.Exec(ctx, sql, currentBalance, login)
	if err != nil {
		tx.Rollback(ctx)
		return false, err
	}
	err = tx.Commit(ctx)
	if err != nil {
		tx.Rollback(ctx)
		return false, err
	}
	return true, nil
}

func (store SQLStore) GetUserBalance(data UserData, ctx context.Context) (BalanceResponce, error) {
	var sql string
	var result BalanceResponce
	if userLogin, ok := ctx.Value(UserLoginCtxKey).(string); ok {
		sql = fmt.Sprintf(`SELECT accrual_points, withdrawal FROM users WHERE login = '%s'`, userLogin)

		err := store.DB.QueryRow(ctx, sql).Scan(&result.Accrual, &result.Withdrawn)
		if err != nil {
			return result, err
		}
		return result, nil
	}
	err := errors.New("no login in context")
	return result, err
}

func (store SQLStore) WitdrawFromUser(userData UserData, withdraw WithdrawRequest, ctx context.Context) error {
	if userLogin, ok := ctx.Value(UserLoginCtxKey).(string); ok {
		currentBalance := userData.AccrualPoints

		currentBalance -= int(withdraw.Amount * 100)
		currentWithdrawn := userData.Withdrawal + int(withdraw.Amount*100)
		sql := `UPDATE users SET accrual_points = $1, withdrawal = $2 WHERE login = $3`
		tx, err := store.DB.Begin(ctx)
		if err != nil {
			return err
		}
		_, err = store.DB.Exec(ctx, sql, currentBalance, currentWithdrawn, userLogin)
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
	err := errors.New("no userLogin in context")
	return err
}

func (store SQLStore) GetWithdrawals(userData UserData) ([]WithdrawResponse, error) {
	var result []WithdrawResponse
	sqlQuery := fmt.Sprintf(`SELECT order_number, withdrawal, created FROM orders WHERE customer = '%s' and withdrawal > 0 ORDER BY id DESC`, userData.Login)
	ctx := context.Background()
	rows, err := store.DB.Query(ctx, sqlQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var order WithdrawResponse
		if err := rows.Scan(&order.OrderNumber, &order.Amount, &order.ProcessedAt); err != nil {
			return result, err
		}
		order.Amount = order.Amount / 100
		result = append(result, order)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}
