package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func RangeQuery(scaleInterval time.Duration, step time.Duration) {
	client, err := api.NewClient(api.Config{
		Address: "http://localhost:16292",
	})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := v1.Range{
		Start: time.Now().Add(-scaleInterval),
		End:   time.Now(),
		Step:  step,
	}
	result, warnings, err := v1api.QueryRange(ctx, "avg by(pod) (irate(container_cpu_usage_seconds_total{job=\"kube_sd\", metrics_topic!=\"\", pod!=\"\"}[1m]))", r, v1.WithTimeout(5*time.Second))
	if err != nil {
		fmt.Printf("Error querying Prometheus: %v\n", err)
		os.Exit(1)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	fmt.Printf("Result:\n%v\n", result)
	// matrix := result.(model.Matrix)
	// matrix.
}

func main() {
	RangeQuery(5*time.Minute, 10*time.Second)
}
