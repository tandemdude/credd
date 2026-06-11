package server

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoggingUnaryInterceptor logs each unary RPC call with its outcome
func LoggingUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	code := codes.OK
	if err != nil {
		code = status.Code(err)
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	slog.InfoContext(
		ctx,
		"Call Finished",
		slog.String("method", info.FullMethod),
		slog.Duration("duration", time.Since(start)),
		slog.String("code", code.String()),
		slog.String("err", errStr),
	)

	return resp, err
}
