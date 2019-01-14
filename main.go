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
