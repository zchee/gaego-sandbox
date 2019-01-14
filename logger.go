// Copyright 2019 The gaego-sandbox Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"net/http"

	"github.com/zchee/zap-encoder/stackdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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
