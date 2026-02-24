package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/undiegomejia/flow/contrib/orm/gorm"
)

func main() {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		fmt.Println("PG_DSN not set; example will exit")
		return
	}
	g, err := gormadapter.ConnectPostgres(dsn)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer g.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := g.Ping(ctx); err != nil {
		log.Fatalf("ping: %v", err)
	}
	fmt.Println("connected to Postgres successfully")
}
