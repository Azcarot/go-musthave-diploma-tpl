package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/Azcarot/GopherMarketProject/internal/storage"
)

func CheckAuthorization(h http.Handler) http.Handler {
	login := func(res http.ResponseWriter, req *http.Request) {
		token := req.Header.Get("Authorization")
		claims, ok := storage.VerifyToken(token)
		if !ok {
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		var userData storage.UserData
		userData.Login = claims["sub"].(string)
		ok, err := storage.PgxStorage.CheckUserExists(storage.ST, userData)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !ok {
			res.WriteHeader(http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(context.WithValue(req.Context(), storage.UserLoginCtxKey, userData.Login), 1000*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)
		h.ServeHTTP(res, req)
	}
	return http.HandlerFunc(login)
}
