package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/store"
)

func TestTaskByPathNotFound(t *testing.T) {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()

	e := New(nil, nil, st, nil, nil, nil)

	_, err = e.TaskByPath(context.Background(), model.CanonicalPath("missing"))
	if err == nil {
		t.Fatal("TaskByPath() error = nil, want not found")
	}
	if !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("TaskByPath() error = %v, want ErrTaskNotFound", err)
	}
}
