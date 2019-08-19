package timber

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/evergreen-ci/pail"
	"github.com/evergreen-ci/poplar"
	"github.com/evergreen-ci/timber/internal"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	rpcAddress = "localhost:2289"
	dbName     = "poplar-benchmark"
	bucketName = "pail-s3-test"
	cedar      = "./cedar"
	lineLength = 200
)

// RunBasicSenderBenchmark runs a poplar benchmark suite for timber's basic
// sender implementation.
func RunBasicSenderBenchmark(ctx context.Context) error {
	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := setupBenchmark(srvCtx); err != nil {
		return err
	}
	defer func() {
		if err := teardownBenchmark(srvCtx); err != nil {
			grip.Error(err)
		}
	}()

	prefix := fmt.Sprintf("basic_sender_benchmark_report_%d", time.Now().Unix())
	if err := os.Mkdir(prefix, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to create top level directory")
	}

	logSizes := []int{1e5, 1e7}
	var combinedReports string
	for _, logSize := range logSizes {
		suitePrefix := filepath.Join(prefix, fmt.Sprintf("%d", logSize))
		if err := os.Mkdir(suitePrefix, os.ModePerm); err != nil {
			return errors.Wrap(err, "failed to create subdirectory")
		}

		suite := getBasicSenderSuite(logSize)
		results, err := suite.Run(ctx, suitePrefix)
		if err != nil {
			combinedReports += fmt.Sprintf("Log Size: %d\n===============\nError:\n%s\n", logSize, err)
			continue
		}

		combinedReports += fmt.Sprintf("Log Size: %d\n===============\n%s\n", logSize, results.Report())
	}

	f, err := os.Create(filepath.Join(prefix, "results.txt"))
	if err != nil {
		return errors.Wrap(err, "problem creating new file")
	}
	defer f.Close()

	_, err = f.WriteString(combinedReports)
	if err != nil {
		return errors.Wrap(err, "problem writing to file")
	}

	return nil
}

// RunFlushBenchmark runs a poplar benchmark suite for the underlying flush
// function used in timber's basic sender implementation.
func RunFlushBenchmark(ctx context.Context) error {
	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := setupBenchmark(srvCtx); err != nil {
		return err
	}
	defer func() {
		if err := teardownBenchmark(srvCtx); err != nil {
			grip.Error(err)
		}
	}()

	prefix := fmt.Sprintf("flush_benchmark_report_%d", time.Now().Unix())
	if err := os.Mkdir(prefix, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to create top level directory")
	}

	var report string
	suite := getFlushSuite()
	results, err := suite.Run(ctx, prefix)
	if err != nil {
		report = fmt.Sprintf("Error:\n%s", err)
	} else {
		report = results.Report()
	}

	f, err := os.Create(filepath.Join(prefix, "results.txt"))
	if err != nil {
		return errors.Wrap(err, "problem creating new file")
	}
	defer f.Close()

	_, err = f.WriteString(report)
	if err != nil {
		return errors.Wrap(err, "problem writing to file")
	}

	return nil
}

func setupBenchmark(ctx context.Context) error {
	args := []string{
		"admin",
		"conf",
		"load",
		"--file",
		filepath.Join("testdata", "cedarconf.yaml"),
		"--dbName",
		dbName,
	}
	cmd := exec.Command(cedar, args...)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "problem loading cedar config")
	}

	args = []string{
		"service",
		"--dbName",
		dbName,
	}
	cmd = exec.CommandContext(ctx, cedar, args...)
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "problem starting local cedar service")
	}

	return nil
}

func teardownBenchmark(ctx context.Context) error {
	catcher := grip.NewBasicCatcher()

	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		catcher.Add(errors.Wrap(err, "problem connecting to mongo"))
	} else {
		catcher.Add(errors.Wrap(client.Database(dbName).Drop(ctx), "problem dropping db"))
	}

	opts := pail.S3Options{
		Name:   bucketName,
		Region: "us-east-1",
	}
	bucket, err := pail.NewS3Bucket(opts)
	if err != nil {
		catcher.Add(errors.Wrap(err, "problem connecting to s3"))
	} else {
		catcher.Add(errors.Wrap(bucket.RemovePrefix(ctx, ""), "problem removing s3 files"))
	}

	return catcher.Resolve()
}

func getBasicSenderSuite(logSize int) poplar.BenchmarkSuite {
	return poplar.BenchmarkSuite{
		{
			CaseName:         "100KBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e5),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "1MBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e6),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "10MBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e7),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "100MBBuffer",
			Bench:            getBasicSenderBenchmark(logSize, 1e8),
			MinRuntime:       time.Millisecond,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            1,
			MinIterations:    1,
			MaxIterations:    2,
			Recorder:         poplar.RecorderPerf,
		},
	}
}

func getBasicSenderBenchmark(logSize, maxBufferSize int) poplar.Benchmark {
	numLines := logSize / lineLength
	if numLines == 0 {
		numLines = 1
	}
	lines := make([]string, numLines)
	for i := 0; i < numLines; i++ {
		lines[i] = newRandCharSetString(lineLength)
	}

	return func(ctx context.Context, r poplar.Recorder, _ int) error {
		opts := &LoggerOptions{
			Project:       "poplar-benchmarking",
			TaskID:        newUUID(),
			RPCAddress:    rpcAddress,
			Insecure:      true,
			MaxBufferSize: maxBufferSize,
		}

		logger, err := MakeLogger(ctx, "benchmark", opts)
		if err != nil {
			return errors.Wrap(err, "problem creating buildlogger sender")
		}

		r.SetState(0)
		for _, line := range lines {
			if err = ctx.Err(); err != nil {
				return errors.Wrap(err, "context error while sending")
			}
			m := message.ConvertToComposer(level.Debug, line)
			startAt := time.Now()
			r.Begin()
			logger.Send(m)
			r.End(time.Since(startAt))
			r.IncOps(1)
		}

		r.SetState(1)
		startAt := time.Now()
		r.Begin()
		if err = logger.Close(); err != nil {
			return errors.Wrap(err, "problem closing buildlogger sender")
		}
		r.End(time.Since(startAt))
		r.IncOps(1)

		return nil
	}
}

func getFlushSuite() poplar.BenchmarkSuite {
	return poplar.BenchmarkSuite{
		{
			CaseName:         "1KBBuffer",
			Bench:            getFlushBenchmark(1e3),
			MinRuntime:       time.Second,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            10,
			MinIterations:    5,
			MaxIterations:    20,
			Recorder:         poplar.RecorderPerf,
		},
		{
			CaseName:         "10KBBuffer",
			Bench:            getFlushBenchmark(1e4),
			MinRuntime:       time.Second,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            10,
			MinIterations:    5,
			MaxIterations:    20,
			Recorder:         poplar.RecorderPerf,
		},
		{
			CaseName:         "100KBBuffer",
			Bench:            getFlushBenchmark(1e5),
			MinRuntime:       time.Second,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            10,
			MinIterations:    5,
			MaxIterations:    20,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "1MBBuffer",
			Bench:            getFlushBenchmark(1e6),
			MinRuntime:       time.Second,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            10,
			MinIterations:    5,
			MaxIterations:    20,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "10MBBuffer",
			Bench:            getFlushBenchmark(1e7),
			MinRuntime:       time.Second,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            10,
			MinIterations:    5,
			MaxIterations:    20,
			Recorder:         poplar.RecorderPerf,
		},
		{

			CaseName:         "100MBBuffer",
			Bench:            getFlushBenchmark(1e8),
			MinRuntime:       time.Second,
			MaxRuntime:       10 * time.Minute,
			Timeout:          20 * time.Minute,
			IterationTimeout: 10 * time.Minute,
			Count:            10,
			MinIterations:    5,
			MaxIterations:    20,
			Recorder:         poplar.RecorderPerf,
		},
	}
}

func getFlushBenchmark(bufferSize int) poplar.Benchmark {
	numLines := bufferSize / lineLength
	if numLines == 0 {
		numLines = 1
	}
	buffer := make([]*internal.LogLine, numLines)
	for i := 0; i < numLines; i++ {
		ts := time.Now()
		buffer[i] = &internal.LogLine{
			Timestamp: &timestamp.Timestamp{Seconds: ts.Unix(), Nanos: int32(ts.Nanosecond())},
			Data:      newRandCharSetString(lineLength),
		}
	}

	return func(ctx context.Context, r poplar.Recorder, count int) error {
		opts := &LoggerOptions{
			Project:    "poplar-benchmarking",
			TaskID:     newUUID(),
			RPCAddress: rpcAddress,
			Insecure:   true,
		}

		logger, err := MakeLogger(ctx, "benchmark", opts)
		if err != nil {
			return errors.Wrap(err, "problem creating buildlogger sender")
		}

		r.SetState(0)
		b := logger.(*buildlogger)
		for i := 0; i < count; i++ {
			b.buffer = buffer
			startAt := time.Now()
			r.Begin()
			if err := b.flush(); err != nil {
				return errors.Wrap(err, "problem flushing")
			}
			r.End(time.Since(startAt))
			r.IncOps(1)
		}

		r.SetState(1)
		startAt := time.Now()
		r.Begin()
		if err = logger.Close(); err != nil {
			return errors.Wrap(err, "problem closing buildlogger sender")
		}
		r.End(time.Since(startAt))
		r.IncOps(1)

		return nil
	}

}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func newRandCharSetString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
