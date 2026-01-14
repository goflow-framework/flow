package main

import (
	"context"
	"fmt"
	"log"

	"github.com/undiegomejia/flow/pkg/flow"
	"github.com/undiegomejia/flow/pkg/plugins"
	// import the sample plugin so it registers during init()
	_ "github.com/undiegomejia/flow/pkg/plugins/sample"
)

func main() {
	app := flow.New("plugin-demo")

	// ApplyAll will run Init and Mount for registered (compile-time) plugins
	if err := plugins.ApplyAll(app); err != nil {
		log.Fatalf("apply plugins: %v", err)
	}

	// read service registered by the sample plugin
	if v, ok := app.GetService("sample.value"); ok {
		fmt.Printf("sample.value=%v\n", v)
	} else {
		fmt.Println("sample.value not found")
	}

	// shutdown (no server started in this simple demo)
	_ = app.Shutdown(context.Background())
}
