package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Azcarot/GopherMarketProject/internal/storage"
	"github.com/Azcarot/GopherMarketProject/internal/utils"
)

type OrderRequest struct {
	OrderNumber string  `json:"order"`
	Status      string  `json:"status"`
	Accrual     float64 `json:"accrual"`
}

func Order(res http.ResponseWriter, req *http.Request) {
	var userData storage.UserData
	var ctx context.Context
	var ctxOrderKey storage.CtxKey
	ctx = context.Background()
	dataLogin, ok := req.Context().Value(storage.UserLoginCtxKey).(string)
	if !ok {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	userData.Login = dataLogin
	data, err := io.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	asString := string(data)

	orderNumber, err := strconv.ParseUint(asString, 10, 64)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	ok = utils.IsOrderNumberValid(orderNumber)
	if !ok {
		res.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	var order storage.OrderData
	ctxOrderKey = storage.OrderNumberCtxKey
	ctx = context.WithValue(ctx, ctxOrderKey, orderNumber)
	order.OrderNumber = orderNumber
	order.User = userData.Login
	mut := sync.Mutex{}
	mut.Lock()
	defer mut.Unlock()
	ok, anotherUser, err := storage.PgxStorage.CheckIfOrderExists(storage.ST, ctx, order)
	if errors.Is(err, errors.New("no order number in context")) {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	if anotherUser {
		res.WriteHeader(http.StatusConflict)
		return
	}
	if !ok {
		res.WriteHeader(http.StatusOK)
		return
	}
	order.Date = time.Now().Format(time.RFC3339)
	err = storage.PgxStorage.CreateNewOrder(storage.ST, ctx, order)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusAccepted)
}
