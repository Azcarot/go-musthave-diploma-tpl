package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Azcarot/GopherMarketProject/internal/router"
	"github.com/Azcarot/GopherMarketProject/internal/storage"
	"github.com/Azcarot/GopherMarketProject/internal/utils"
)

func main() {
	flag := utils.ParseFlagsAndENV()
	if flag.FlagDBAddr != "" {
		err := storage.NewConn(flag)
		if err != nil {
			panic(err)
		}
		storage.PgxStorage.CreateTablesForGopherStore(storage.ST)
		defer storage.DB.Close(context.Background())
		r := router.MakeRouter(flag)
		server := &http.Server{
			Addr:    flag.FlagAddr,
			Handler: r,
		}
		server.ListenAndServe()

	}
	fmt.Fprintf(os.Stderr, "Missing required flag -d : DataBase address\n")
}
