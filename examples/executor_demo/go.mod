module github.com/goflow-framework/flow/examples/executor_demo

go 1.24.0

toolchain go1.24.11

require (
	github.com/alicebob/miniredis/v2 v2.22.0
	github.com/redis/go-redis/v9 v9.17.2
)

require (
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v0.0.0-20210529063254-f4c35e4016d9 // indirect
)

replace github.com/goflow-framework/flow => ../../
