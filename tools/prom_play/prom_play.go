package main

import (
	"github.com/tikv/pd/autoscale"
)

func main() {
	// RangeQuery(5*time.Minute, 10*time.Second)
	cli, err := autoscale.NewPromClient("http://localhost:16292")
	if err != nil {
		panic(err)
	}
	cli.QueryCpu()
}
