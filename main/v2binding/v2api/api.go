package main

import (
	"time"

	"github.com/google/gops/agent"
	"github.com/v2fly/v2ray-core/v5/main/v2binding"
)

func main() {
	v2binding.StartAPIInstance(".")
	if err := agent.Listen(agent.Options{
		Addr: "127.0.0.1:1000",
	}); err != nil {
		panic(err)
	}
	for {
		time.Sleep(time.Minute)
	}
}
