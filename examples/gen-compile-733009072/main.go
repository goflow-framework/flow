package main

import (
	"context"
	"fmt"
	"log"

	models "github.com/dministrator/flow/examples/gen-compile-733009072/app/models"
	orm "github.com/dministrator/flow/internal/orm"
	flow "github.com/dministrator/flow/pkg/flow"
	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()
	adapter, err := orm.Connect("file::memory:?cache=shared")
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer adapter.Close()

	app := flow.New("gen-compile", flow.WithBun(adapter))
	if err := flow.AutoMigrate(ctx, app, (*models.Post)(nil)); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	p := &models.Post{Title: "compile-test-hello"}
	if err := p.Save(ctx, app); err != nil {
		log.Fatalf("save: %v", err)
	}
	var got models.Post
	if err := flow.FindByPK(ctx, app, &got, p.ID); err != nil {
		log.Fatalf("find: %v", err)
	}
	fmt.Println("FOUND:", got.Title)

	if err := p.Delete(ctx, app); err != nil {
		log.Fatalf("delete: %v", err)
	}
}
