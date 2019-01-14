// Copyright 2019 The gaego-sandbox Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command gaego-sandbox is the sample logging-quickstart writes a log entry to Stackdriver Logging.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"cloud.google.com/go/logging"
	"github.com/zchee/zap-encoder/stackdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/genproto/googleapis/api/monitoredres"
)

const logName = "app_logs"

var (
	projectID    string
	requestCount int32
	monRes       *monitoredres.MonitoredResource
)

func init() {
	if err := zap.RegisterEncoder("stackdriver", func(cfg zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return &stackdriver.Encoder{
			Encoder: zapcore.NewJSONEncoder(cfg),
		}, nil
	}); err != nil {
		panic(err)
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	monRes = &monitoredres.MonitoredResource{
		Labels: map[string]string{
			"module_id":  os.Getenv("GAE_SERVICE"),
			"project_id": projectID,
			"version_id": os.Getenv("GAE_VERSION"),
		},
		Type: "gae_app",
	}

	zl := NewLogger(zap.NewAtomicLevelAt(zapcore.DebugLevel))
	defer zl.Sync()

	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	mux.HandleFunc("/nolog", nolog)

	s := http.Server{
		// TODO(zchee): switch to `apply` way.
		Handler: Adapter(zl)(mux),
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	l, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen %s: %v", port, err)
	}
	log.Printf("Listening on port: %s\n", port)

	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(l)
	}()

	for {
		select {
		case <-ctx.Done():
			s.Shutdown(ctx)
			l.Close()
			return
		case err := <-errc:
			log.Fatal(err)
		}
	}
}

func traceID(r *http.Request) string {
	return fmt.Sprintf("projects/%s/traces/%s", projectID, strings.Split(r.Header.Get("X-Cloud-Trace-Context"), "/")[0])
}

func newClient(ctx context.Context) *logging.Client {
	client, err := logging.NewClient(ctx, fmt.Sprintf("projects/%s", projectID))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	return client
}

func index(w http.ResponseWriter, r *http.Request) {
	defer func() {
		// avoid race
		atomic.AddInt32(&requestCount, 1)
	}()

	ctx := r.Context()
	zl := FromContext(ctx).Named("index")

	client := newClient(ctx)
	defer client.Close()

	// TODO(zchee): not support yet configure `logging.Entry`.
	// trace := traceID(r)

	zl.Info(fmt.Sprintf("[request #%d] First entry", requestCount))

	zl.Warn(fmt.Sprintf("[request #%d] A second entry here!", requestCount))
}

func nolog(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "No Logged: %v\n")
}

func otherFunc() {
	log.Printf("otherFunc output log")
}

type ctxZapLogger struct{}

var (
	ctxZapLoggerKey = &ctxZapLogger{}
)

// NewLogger returns the new zap.Logger with stackdriver encoder.
func NewLogger(atomlv zap.AtomicLevel, opts ...zap.Option) *zap.Logger {
	var zopts []zap.Option

	cfg := stackdriver.NewStackdriverConfig()
	switch lv := atomlv.Level(); lv {
	default:
		// nothig to do
	case zap.DebugLevel:
		zopts = append(zopts, zap.AddCallerSkip(0), zap.AddStacktrace(lv))
	}
	cfg.Level = atomlv

	zopts = append(zopts, opts...)
	zl, err := cfg.Build(zopts...)
	if err != nil {
		panic(zl)
	}

	return zl
}

func newContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxZapLoggerKey, logger)
}

// FromContext extract zap.Logger from the context.
func FromContext(ctx context.Context) *zap.Logger {
	l, ok := ctx.Value(ctxZapLoggerKey).(*zap.Logger)
	if !ok {
		return zap.NewNop()
	}

	return l
}

// WithContext inserts the zap.Logger into context.
func WithContext(ctx context.Context, fields ...zapcore.Field) context.Context {
	return newContext(ctx, FromContext(ctx).With(fields...))
}

// Adapter injects the zap.Logger context into http.Request.Context.
func Adapter(l *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(newContext(r.Context(), l))

			next.ServeHTTP(w, r)
		})
	}
}
