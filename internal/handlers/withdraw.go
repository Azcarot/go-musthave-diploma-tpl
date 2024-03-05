package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Azcarot/GopherMarketProject/internal/storage"
	"github.com/Azcarot/GopherMarketProject/internal/utils"
)

func Withdraw(res http.ResponseWriter, req *http.Request) {
	var userData storage.UserData
	var orderData storage.OrderData
	var ctxOrderKey, ctxUserKey storage.CtxKey
	ctx := req.Context()
	dataLogin, ok := ctx.Value(storage.UserLoginCtxKey).(string)
	if !ok {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	userData.Login = dataLogin

	withdrawalData := storage.WithdrawRequest{}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(data, &withdrawalData); err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	orderNumber, err := strconv.ParseUint(withdrawalData.OrderNumber, 10, 64)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	ok = utils.IsOrderNumberValid(orderNumber)
	if !ok {
		res.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	ctxOrderKey = storage.OrderNumberCtxKey
	ctxUserKey = storage.UserLoginCtxKey
	ctx = context.WithValue(ctx, ctxOrderKey, orderNumber)
	ctx = context.WithValue(ctx, ctxUserKey, userData.Login)
	mut := sync.Mutex{}
	mut.Lock()
	defer mut.Unlock()
	err = storage.PgxStorage.WithdrawFromUser(storage.ST, ctx, withdrawalData)
	if errors.Is(err, fmt.Errorf("payment required")) {
		res.WriteHeader(http.StatusPaymentRequired)
		return
	}
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	orderData.Accrual = 0
	orderData.OrderNumber, _ = strconv.ParseUint(withdrawalData.OrderNumber, 10, 64)
	orderData.Withdrawal = int(withdrawalData.Amount * 100)
	orderData.Date = time.Now().Format(time.RFC3339)
	orderData.User = userData.Login
	err = storage.PgxStorage.CreateNewOrder(storage.ST, ctx, orderData)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
}
