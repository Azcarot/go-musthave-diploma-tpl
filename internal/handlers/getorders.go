package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Azcarot/GopherMarketProject/internal/storage"
	"github.com/Azcarot/GopherMarketProject/internal/utils"
	"github.com/jackc/pgx/v5"
)

func GetOrders(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	_, ok := req.Context().Value(storage.UserLoginCtxKey).(string)
	if !ok {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	orders, err := storage.PgxStorage.GetCustomerOrders(storage.ST, ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		res.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	result, err := json.Marshal(orders)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(result)

}

func GetOrderData(flag utils.Flags, order uint64) (OrderRequest, error) {
	pth := flag.FlagAccrualAddr + "/api/orders/" + strconv.Itoa(int(order))
	var b []byte
	result := OrderRequest{}
	resp, err := http.NewRequest("GET", pth, bytes.NewBuffer(b))
	if err != nil {
		return result, err
	}

	var res *http.Response
	res, err = CheckStatus(resp)
	if err != nil {
		return result, err
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return result, err
	}
	defer res.Body.Close()

	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, err
}

func CheckStatus(resp *http.Request) (*http.Response, error) {
	client := &http.Client{}
	res, err := client.Do(resp)
	if err != nil {
		return res, err
	}
	if res.StatusCode == http.StatusTooManyRequests {
		time.Sleep(time.Duration(1 * time.Second))
		res, _ = CheckStatus(resp)

	}
	return res, err
}

func ActualiseOrders(flag utils.Flags) {
	orderNumbers, err := storage.PgxStorage.GetUnfinishedOrders(storage.ST)
	ctx := context.Background()
	if err != nil {
		time.Sleep(time.Duration(time.Duration(5).Seconds()))
		orderNumbers, err = storage.PgxStorage.GetUnfinishedOrders(storage.ST)
		if err != nil {
			return
		}
	}
	var wg sync.WaitGroup
	for i, order := range orderNumbers {
		ind := i
		ord := order
		wg.Add(1)
		go func(int, uint64) {
			defer wg.Done()
			orderReq, err := GetOrderData(flag, ord)
			if err != nil {
				return
			}
			if (orderReq.Status != "NEW") && (orderReq.Status != "PROCESSING") {
				var orderData storage.OrderData
				orderData.Accrual = int(orderReq.Accrual * 100)
				orderNumber, err := strconv.Atoi(orderReq.OrderNumber)
				if err != nil {
					return
				}
				orderData.OrderNumber = uint64(orderNumber)
				orderData.State = orderReq.Status
				err = storage.PgxStorage.UpdateOrder(storage.ST, ctx, orderData)
				if err != nil {
					return
				}
				if orderData.Accrual > 0 {
					_, err := storage.PgxStorage.AddBalanceToUser(storage.ST, orderData)
					if err != nil {
						return
					}
				}
				orderNumbers[ind] = orderNumbers[len(orderNumbers)-1]
				orderNumbers = orderNumbers[:len(orderNumbers)-1]
			}
			if len(orderNumbers) == 0 {
				return
			}
		}(ind, ord)
	}
	wg.Wait()

}
