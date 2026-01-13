package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	controllers "github.com/dministrator/flow/examples/simple/app/controllers"
	flow "github.com/dministrator/flow/pkg/flow"
	"github.com/dministrator/flow/pkg/plugins"
)

func main() {
	app := flow.New("examples-simple")

	// point views to the example's views directory
	app.Views = flow.NewViewManager("examples/simple/app/views")

	// router and controller
	r := flow.NewRouter(app)
	users := controllers.NewUsersController(app)
	if err := r.Resources("users", users); err != nil {
		fmt.Fprintf(os.Stderr, "register resources: %v\n", err)
		os.Exit(1)
	}

	app.SetRouter(r.Handler())

	// apply registered plugins before starting
	if err := plugins.ApplyAll(app); err != nil {
		fmt.Fprintf(os.Stderr, "apply plugins: %v\n", err)
		os.Exit(1)
	}

	// start server
	if err := app.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start error: %v\n", err)
		os.Exit(1)
	}

	// wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	_ = app.Shutdown(context.Background())
}
