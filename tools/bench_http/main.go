package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"github.com/dministrator/flow/pkg/flow"
)

func main() {
	var (
		addr        = flag.String("addr", ":8080", "server address")
		durationSec = flag.Int("duration", 5, "load duration seconds")
		concurrency = flag.Int("concurrency", 50, "number of concurrent clients")
		poolEnabled = flag.Bool("pool", true, "enable flow.UseContextPool")
	)
	flag.Parse()

	flow.UseContextPool = *poolEnabled
	log.Printf("UseContextPool=%v", flow.UseContextPool)

	r := flow.NewRouter(nil)
	r.Get("/ping", func(ctx *flow.Context) {
		_ = ctx.JSON(200, map[string]string{"ok": "1"})
	})

	srv := &http.Server{Addr: *addr, Handler: r.Handler()}

	go func() {
		log.Printf("starting server on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// wait briefly for server to be ready
	time.Sleep(200 * time.Millisecond)

	// prepare profiling files
	cpuFile := fmt.Sprintf("bench_cpu_pool_%v.prof", *poolEnabled)
	heapFile := fmt.Sprintf("bench_heap_pool_%v.prof", *poolEnabled)
	cf, err := os.Create(cpuFile)
	if err != nil {
		log.Fatalf("create cpu prof: %v", err)
	}
	defer cf.Close()

	if err := pprof.StartCPUProfile(cf); err != nil {
		log.Fatalf("start cpu profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	// run load
	dur := time.Duration(*durationSec) * time.Second
	end := time.Now().Add(dur)
	var total int64
	var success int64

	client := &http.Client{Timeout: 5 * time.Second}

	wg := make(chan struct{})
	for i := 0; i < *concurrency; i++ {
		go func() {
			for time.Now().Before(end) {
				resp, err := client.Get("http://127.0.0.1" + *addr + "/ping")
				atomic.AddInt64(&total, 1)
				if err == nil {
					if resp.StatusCode == 200 {
						atomic.AddInt64(&success, 1)
					}
					resp.Body.Close()
				}
			}
			wg <- struct{}{}
		}()
	}

	// wait for goroutines
	for i := 0; i < *concurrency; i++ {
		<-wg
	}

	// write heap profile
	hf, err := os.Create(heapFile)
	if err != nil {
		log.Fatalf("create heap prof: %v", err)
	}
	defer hf.Close()
	if err := pprof.WriteHeapProfile(hf); err != nil {
		log.Fatalf("write heap prof: %v", err)
	}

	elapsed := time.Duration(*durationSec) * time.Second
	rps := float64(total) / elapsed.Seconds()
	succRate := float64(success) / float64(total) * 100.0

	log.Printf("done: total=%d success=%d rps=%.2f success_rate=%.2f%%", total, success, rps, succRate)

	// gracefully shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	// trap Ctrl-C while program exits
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	select {
	case <-c:
		log.Printf("received interrupt, exiting")
	case <-time.After(100 * time.Millisecond):
	}
}
