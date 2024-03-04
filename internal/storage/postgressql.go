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
	CheckUserExists(data UserData) (bool, error)
	CheckUserPassword(data UserData) (bool, error)
	CreateNewOrder(data OrderData, ctx context.Context) error
	UpdateOrder(data OrderData) error
	AddBalanceToUser(orderData OrderData) (bool, error)
	WitdrawFromUser(userData UserData, withdraw WithdrawRequest, ctx context.Context) error
	GetUserBalance(data UserData, ctx context.Context) (BalanceResponce, error)
	GetWithdrawals(userData UserData) ([]WithdrawResponse, error)
	GetCustomerOrders(login string) ([]OrderResponse, error)
	CheckIfOrderExists(data OrderData, ctx context.Context) (bool, bool, error)
	GetUnfinishedOrders() ([]uint64, error)
}

type SQLStore struct {
	DB *pgx.Conn
}

var DB *pgx.Conn
var ST PgxStorage

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
	ST = MakeStore(DB)
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

func (store SQLStore) CreateTablesForGopherStore() {
	ctx := context.Background()
	mut.Lock()
	defer mut.Unlock()
	queryForFun := `DROP TABLE IF EXISTS users CASCADE`
	store.DB.Exec(ctx, queryForFun)
	query := `CREATE TABLE IF NOT EXISTS users (
		id SERIAL NOT NULL PRIMARY KEY, 
		login text NOT NULL, 
		password text NOT NULL, 
		accrual_points bigint NOT NULL, 
		withdrawal BIGINT NOT NULL,
		created text )`

	_, err := store.DB.Exec(ctx, query)

	if err != nil {

		log.Printf("Error %s when creating user table", err)

	}
	queryForFun = `DROP TABLE IF EXISTS orders CASCADE`
	store.DB.Exec(ctx, queryForFun)
	query = `CREATE TABLE IF NOT EXISTS orders(
		id SERIAL NOT NULL PRIMARY KEY,
		order_number BIGINT,
		accrual_points BIGINT NOT NULL,
		state TEXT,
		withdrawal BIGINT NOT NULL,
		customer TEXT NOT NULL,
		created TEXT
	)`
	_, err = store.DB.Exec(ctx, query)

	if err != nil {

		log.Printf("Error %s when creating order table", err)

	}
}
