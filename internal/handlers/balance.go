package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Azcarot/GopherMarketProject/internal/storage"
)

func GetBalance(res http.ResponseWriter, req *http.Request) {
	var userData storage.UserData
	ctx := req.Context()
	data, ok := ctx.Value(storage.UserLoginCtxKey).(string)
	if !ok {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	userData.Login = data
	var balanceData storage.BalanceResponce
	balanceData, err := storage.PgxStorage.GetUserBalance(storage.ST, ctx, userData)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	balanceData.Accrual = balanceData.Accrual / 100
	balanceData.Withdrawn = balanceData.Withdrawn / 100
	result, err := json.Marshal(balanceData)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(result)
}
