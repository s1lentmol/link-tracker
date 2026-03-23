package grpcx

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func UnaryServerLogger(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		startedAt := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(startedAt)
		code := status.Code(err)

		attrs := []slog.Attr{
			slog.String("rpc_method", info.FullMethod),
			slog.String("status_code", code.String()),
			slog.Int64("duration_ms", duration.Milliseconds()),
		}
		if err != nil {
			logger.LogAttrs(ctx, slog.LevelError, "grpc server request failed", append(attrs, slog.String("error", err.Error()))...)
		} else {
			logger.LogAttrs(ctx, slog.LevelInfo, "grpc server request handled", attrs...)
		}

		return resp, err
	}
}

func UnaryClientLogger(logger *slog.Logger, target string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		startedAt := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(startedAt)
		code := status.Code(err)

		attrs := []slog.Attr{
			slog.String("rpc_method", method),
			slog.String("target", target),
			slog.String("status_code", code.String()),
			slog.Int64("duration_ms", duration.Milliseconds()),
		}
		if err != nil {
			logger.LogAttrs(ctx, slog.LevelError, "grpc client request failed", append(attrs, slog.String("error", err.Error()))...)
		} else {
			logger.LogAttrs(ctx, slog.LevelInfo, "grpc client request handled", attrs...)
		}

		return err
	}
}
