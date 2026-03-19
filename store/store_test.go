package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jpcummins/tsk-lib/model"
)

func TestTaskByPathNotFound(t *testing.T) {
	t.Helper()

	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	_, err = s.TaskByPath(context.Background(), model.CanonicalPath("missing"))
	if err == nil {
		t.Fatal("TaskByPath() error = nil, want not found")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("TaskByPath() error = %v, want ErrTaskNotFound", err)
	}
}
