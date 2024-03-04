package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Azcarot/GopherMarketProject/internal/utils"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const SecretKey string = "super-secret"

type MyCustomClaims struct {
	jwt.MapClaims
}

type CtxKey string

var ST = MakeStore(DB)
var mut sync.Mutex

const UserLoginCtxKey CtxKey = "userLogin"
const OrderNumberCtxKey CtxKey = "orderNumber"
const DBCtxKey CtxKey = "dbConn"

type UserData struct {
	Login         string
	Password      string
	AccrualPoints int
	Withdrawal    int
	Date          string
}
type OrderData struct {
	OrderNumber uint64 `json:"number"`
	Accrual     int    `json:"accrual"`
	User        string
	State       string `json:"status"`
	Date        string `json:"uploaded_at"`
	Withdrawal  int
}

type OrderResponse struct {
	OrderNumber string  `json:"number"`
	Accrual     float64 `json:"accrual"`
	State       string  `json:"status"`
	Date        string  `json:"uploaded_at"`
}

type BalanceResponce struct {
	Accrual   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type PgxStorage interface {
	CreateTablesForGopherStore()
	CreateNewUser(data UserData) error
}

type SQLStore struct {
	DB *pgx.Conn
}

var DB *pgx.Conn

type pgxConnTime struct {
	attempts          int
	timeBeforeAttempt int
}

type WithdrawRequest struct {
	OrderNumber string  `json:"order"`
	Amount      float64 `json:"sum"`
}

type WithdrawResponse struct {
	OrderNumber string  `json:"order"`
	Amount      float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

func MakeStore(db *pgx.Conn) PgxStorage {
	return &SQLStore{
		DB: db,
	}
}

func NewConn(f utils.Flags) error {
	var err error
	var attempts pgxConnTime
	attempts.attempts = 3
	attempts.timeBeforeAttempt = 1
	err = connectToDB(f)
	for err != nil {
		//если ошибка связи с бд, то это не эскпортируемый тип, отличный от PgError
		var pqErr *pgconn.PgError
		if errors.Is(err, pqErr) {
			return err

		}
		if attempts.attempts == 0 {
			return err
		}
		times := time.Duration(attempts.timeBeforeAttempt)
		time.Sleep(times * time.Second)
		attempts.attempts -= 1
		attempts.timeBeforeAttempt += 2
		err = connectToDB(f)

	}
	return nil
}

func connectToDB(f utils.Flags) error {
	var err error
	ps := fmt.Sprintf(f.FlagDBAddr)
	DB, err = pgx.Connect(context.Background(), ps)
	return err
}

func CheckDBConnection() http.Handler {
	checkConnection := func(res http.ResponseWriter, req *http.Request) {

		err := DB.Ping(context.Background())
		result := (err == nil)
		if result {
			res.WriteHeader(http.StatusOK)
		} else {
			res.WriteHeader(http.StatusInternalServerError)
		}

	}
	return http.HandlerFunc(checkConnection)
}

func (Store *SQLStore) CreateTablesForGopherStore() {
	ctx := context.Background()
	mut.Lock()
	defer mut.Unlock()
	queryForFun := `DROP TABLE IF EXISTS users CASCADE`
	Store.DB.Exec(ctx, queryForFun)
	query := `CREATE TABLE IF NOT EXISTS users (
		id SERIAL NOT NULL PRIMARY KEY, 
		login text NOT NULL, 
		password text NOT NULL, 
		accrual_points bigint NOT NULL, 
		withdrawal BIGINT NOT NULL,
		created text )`

	_, err := Store.DB.Exec(ctx, query)

	if err != nil {

		log.Printf("Error %s when creating user table", err)

	}
	queryForFun = `DROP TABLE IF EXISTS orders CASCADE`
	Store.DB.Exec(ctx, queryForFun)
	query = `CREATE TABLE IF NOT EXISTS orders(
		id SERIAL NOT NULL PRIMARY KEY,
		order_number BIGINT,
		accrual_points BIGINT NOT NULL,
		state TEXT,
		withdrawal BIGINT NOT NULL,
		customer TEXT NOT NULL,
		created TEXT
	)`
	_, err = Store.DB.Exec(ctx, query)

	if err != nil {

		log.Printf("Error %s when creating order table", err)

	}
}

func (store SQLStore) CreateNewUser(data UserData) error {
	ctx := context.Background()
	encodedPW := utils.ShaData(data.Password, SecretKey)
	mut.Lock()
	defer mut.Unlock()
	_, err := store.DB.Exec(ctx, `INSERT into users (login, password, accrual_points, withdrawal, created) 
	values ($1, $2, $3, $4, $5);`,
		data.Login, encodedPW, 0, 0, data.Date)
	return err
}

func CheckUserExists(db *pgx.Conn, data UserData) (bool, error) {
	ctx := context.Background()
	var login string
	sqlQuery := fmt.Sprintf(`SELECT login FROM users WHERE login = '%s'`, data.Login)
	err := db.QueryRow(ctx, sqlQuery).Scan(&login)

	if errors.Is(err, pgx.ErrNoRows) {

		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil

}

func CheckUserPassword(db *pgx.Conn, data UserData) (bool, error) {
	encodedPw := utils.ShaData(data.Password, SecretKey)
	ctx := context.Background()
	sqlQuery := fmt.Sprintf(`SELECT login, password FROM users WHERE login = '%s'`, data.Login)
	var login, pw string
	err := db.QueryRow(ctx, sqlQuery).Scan(&login, &pw)
	if err != nil {
		return false, err
	}

	if encodedPw != pw {
		return false, nil
	}
	return true, nil
}

func CreateNewOrder(db *pgx.Conn, data OrderData, ctx context.Context) error {
	data.State = "NEW"
	if orderNumber, ok := ctx.Value(OrderNumberCtxKey).(uint64); ok {
		_, err := db.Exec(ctx, `INSERT INTO orders 
	(order_number, accrual_points, state, customer, withdrawal, created) 
	values ($1, $2, $3, $4, $5, $6);`,
			orderNumber, data.Accrual, data.State, data.User, data.Withdrawal, data.Date)
		return err
	} else {
		err := errors.New("no order number in context")
		return err
	}

}

func VerifyToken(token string) (jwt.MapClaims, bool) {
	hmacSecretString := SecretKey
	hmacSecret := []byte(hmacSecretString)
	gettoken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return hmacSecret, nil
	})

	if err != nil {
		return nil, false
	}

	if claims, ok := gettoken.Claims.(jwt.MapClaims); ok && gettoken.Valid {
		return claims, true

	} else {
		log.Printf("Invalid JWT Token")
		return nil, false
	}
}

func GetCustomerOrders(db *pgx.Conn, login string) ([]OrderResponse, error) {
	query := fmt.Sprintf(`SELECT order_number, accrual_points, state, created 
	FROM orders 
	WHERE customer = '%s' 
	ORDER BY id DESC`, login)
	result := []OrderResponse{}
	ctx := context.Background()
	rows, err := db.Query(ctx, query)
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

func CheckIfOrderExists(db *pgx.Conn, data OrderData, ctx context.Context) (bool, bool, error) {
	var query string
	if orderNumber, ok := ctx.Value(OrderNumberCtxKey).(uint64); ok {
		query = fmt.Sprintf(`SELECT order_number, customer 
	FROM orders 
	WHERE order_number = %d`, orderNumber)
		var number uint64
		var login string
		err := db.QueryRow(ctx, query).Scan(&number, &login)
		if errors.Is(err, pgx.ErrNoRows) {
			//No order
			return true, false, err
		}
		// Order exists for another user
		if login != data.User {
			return false, true, err
		}
		// order already exists for current user
		return false, false, err
	}
	err := errors.New("no order number in context")
	return false, false, err
}

func GetUnfinishedOrders(db *pgx.Conn) ([]uint64, error) {
	sqlQuery := "SELECT order_number FROM orders WHERE state IN ('NEW', 'PROCESSING')"
	ctx := context.Background()
	var result []uint64
	rows, err := db.Query(ctx, sqlQuery)
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

func UpdateOrder(db *pgx.Conn, data OrderData) error {
	ctx := context.Background()
	sql := `
	UPDATE orders 
	SET accrual_points = $1, state = $2 
	WHERE order_number = $3;
`
	_, err := db.Exec(ctx, sql, data.Accrual, data.State, data.OrderNumber)
	return err
}

func AddBalanceToUser(db *pgx.Conn, orderData OrderData) (bool, error) {
	ctx := context.Background()
	sqlQuery := fmt.Sprintf(`SELECT users.accrual_points, users.login 
	FROM users
	LEFT JOIN orders  
	ON users.login = orders.customer 
	WHERE orders.order_number = '%d'`, orderData.OrderNumber)
	var currentBalance int
	var login string
	err := db.QueryRow(ctx, sqlQuery).Scan(&currentBalance, &login)
	if err != nil {
		return false, err
	}
	currentBalance += orderData.Accrual
	sql := `UPDATE users SET accrual_points = $1 WHERE login = $2`
	_, err = db.Exec(ctx, sql, currentBalance, login)
	if err != nil {
		return false, err
	}
	return true, err
}

func GetUserBalance(db *pgx.Conn, data UserData, ctx context.Context) (BalanceResponce, error) {
	var sql string
	var result BalanceResponce
	if userLogin, ok := ctx.Value(UserLoginCtxKey).(string); ok {
		sql = fmt.Sprintf(`SELECT accrual_points, withdrawal FROM users WHERE login = '%s'`, userLogin)

		err := db.QueryRow(ctx, sql).Scan(&result.Accrual, &result.Withdrawn)
		if err != nil {
			return result, err
		}
		return result, nil
	}
	err := errors.New("no login in context")
	return result, err
}

func WitdrawFromUser(db *pgx.Conn, userData UserData, withdraw WithdrawRequest, ctx context.Context) error {
	if userLogin, ok := ctx.Value(UserLoginCtxKey).(string); ok {
		currentBalance := userData.AccrualPoints

		currentBalance -= int(withdraw.Amount * 100)
		currentWithdrawn := userData.Withdrawal + int(withdraw.Amount*100)
		sql := `UPDATE users SET accrual_points = $1, withdrawal = $2 WHERE login = $3`
		_, err := db.Exec(ctx, sql, currentBalance, currentWithdrawn, userLogin)
		if err != nil {
			return err
		}
		return nil
	}
	err := errors.New("no userLogin in context")
	return err
}

func GetWithdrawals(db *pgx.Conn, userData UserData) ([]WithdrawResponse, error) {
	var result []WithdrawResponse
	sqlQuery := fmt.Sprintf(`SELECT order_number, withdrawal, created FROM orders WHERE customer = '%s' and withdrawal > 0 ORDER BY id DESC`, userData.Login)
	ctx := context.Background()
	rows, err := db.Query(ctx, sqlQuery)
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
