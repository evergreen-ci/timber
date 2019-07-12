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
	if err := timber.RunBasicSenderBenchmark(ctx); err != nil {
		log.Println(err)
	}

	fmt.Println("running flush benchmark...")
	if err := timber.RunFlushBenchmark(ctx); err != nil {
		log.Println(err)
	}
}
