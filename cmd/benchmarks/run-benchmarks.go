package main

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/timber"
	"github.com/mongodb/grip"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("running basic sender benchmark...")
	if err := timber.RunBasicSenderBenchmark(ctx); err != nil {
		grip.Error(err)
	}

	fmt.Println("running flush benchmark...")
	if err := timber.RunFlushBenchmark(ctx); err != nil {
		grip.Error(err)
	}
}
