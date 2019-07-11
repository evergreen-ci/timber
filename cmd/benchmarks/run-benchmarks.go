package main

import (
	"context"
	"fmt"
	"log"

	"github.com/evergreen-ci/timber"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("running basic sender benchmark...")
	err := timber.RunBasicSenderBenchmark(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
