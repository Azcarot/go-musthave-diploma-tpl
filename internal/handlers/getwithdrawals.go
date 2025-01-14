package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Azcarot/GopherMarketProject/internal/storage"
)

func GetWithdrawals(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	_, ok := req.Context().Value(storage.UserLoginCtxKey).(string)
	if !ok {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	withdrawals, err := storage.PgxStorage.GetWithdrawals(storage.ST, ctx)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	result, err := json.Marshal(withdrawals)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(result)
}
