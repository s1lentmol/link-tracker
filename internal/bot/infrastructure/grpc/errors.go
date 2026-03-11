package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StatusCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}

	var stErr interface{ GRPCStatus() *status.Status }
	if errors.As(err, &stErr) {
		return stErr.GRPCStatus().Code()
	}

	st, ok := status.FromError(err)
	if ok {
		return st.Code()
	}

	return codes.Unknown
}
