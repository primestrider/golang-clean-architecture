package grpc

import (
	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcCode memetakan apperror.Code ke kode status gRPC.
func grpcCode(code apperror.Code) codes.Code {
	switch code {
	case apperror.CodeBadRequest, apperror.CodeValidation:
		return codes.InvalidArgument
	case apperror.CodeUnauthorized:
		return codes.Unauthenticated
	case apperror.CodeForbidden:
		return codes.PermissionDenied
	case apperror.CodeNotFound:
		return codes.NotFound
	case apperror.CodeConflict:
		// Aborted adalah kode gRPC untuk konflik konkurensi, misalnya versi
		// data yang sudah usang (optimistic locking).
		return codes.Aborted
	case apperror.CodePayloadTooLarge:
		return codes.ResourceExhausted
	case apperror.CodeRateLimited:
		return codes.ResourceExhausted
	case apperror.CodeUnavailable:
		return codes.Unavailable
	default:
		return codes.Internal
	}
}

// toStatus mengubah error use case menjadi status gRPC. Error validasi
// menyertakan detail per field (errdetails.BadRequest), setara dengan field
// "errors" di Problem Details pada REST API. Pesan error internal tidak
// pernah dikirim ke client.
func toStatus(err error) error {
	if err == nil {
		return nil
	}
	appErr := apperror.From(err)
	st := status.New(grpcCode(appErr.Code), appErr.Message)
	if len(appErr.Fields) == 0 {
		return st.Err()
	}
	violations := make([]*errdetails.BadRequest_FieldViolation, 0, len(appErr.Fields))
	for _, f := range appErr.Fields {
		violations = append(violations, &errdetails.BadRequest_FieldViolation{Field: f.Field, Description: f.Message})
	}
	if detailed, derr := st.WithDetails(&errdetails.BadRequest{FieldViolations: violations}); derr == nil {
		return detailed.Err()
	}
	return st.Err()
}
