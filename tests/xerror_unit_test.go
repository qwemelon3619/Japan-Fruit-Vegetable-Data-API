package tests

import (
	"errors"
	"testing"

	"japan_data_project/internal/platform/xerror"
)

func TestXErrorNew_ReturnsAppError(t *testing.T) {
	err := xerror.New(xerror.CodeInvalidArgument, "bad request")
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	var appErr *xerror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError type, got=%T", err)
	}
	if appErr.Code != xerror.CodeInvalidArgument {
		t.Fatalf("unexpected code: got=%s", appErr.Code)
	}
	if appErr.Message != "bad request" {
		t.Fatalf("unexpected message: got=%q", appErr.Message)
	}
	if appErr.Unwrap() != nil {
		t.Fatalf("new error must not wrap inner error")
	}
}

func TestXErrorWrap_UnwrapsInnerError(t *testing.T) {
	inner := errors.New("db timeout")
	err := xerror.Wrap(xerror.CodeDB, "query failed", inner)

	var appErr *xerror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError type, got=%T", err)
	}
	if appErr.Code != xerror.CodeDB {
		t.Fatalf("unexpected code: got=%s", appErr.Code)
	}
	if !errors.Is(err, inner) {
		t.Fatalf("wrapped error must satisfy errors.Is for inner error")
	}
}
