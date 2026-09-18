package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

type taskReaderStub struct {
	wait func(context.Context, int64, time.Duration) (*meilisearch.Task, error)
}

func (s taskReaderStub) GetTask(int64) (*meilisearch.Task, error) {
	return nil, nil
}

func (s taskReaderStub) GetTaskWithContext(context.Context, int64) (*meilisearch.Task, error) {
	return nil, nil
}

func (s taskReaderStub) GetTasks(*meilisearch.TasksQuery) (*meilisearch.TaskResult, error) {
	return nil, nil
}

func (s taskReaderStub) GetTasksWithContext(context.Context, *meilisearch.TasksQuery) (*meilisearch.TaskResult, error) {
	return nil, nil
}

func (s taskReaderStub) WaitForTask(int64, time.Duration) (*meilisearch.Task, error) {
	return nil, nil
}

func (s taskReaderStub) WaitForTaskWithContext(ctx context.Context, taskUID int64, interval time.Duration) (*meilisearch.Task, error) {
	return s.wait(ctx, taskUID, interval)
}

func TestWaitTaskReportsTaskOutcome(t *testing.T) {
	underlyingErr := errors.New("transport failed")

	tests := []struct {
		name       string
		task       *meilisearch.Task
		waitErr    error
		wantStatus meilisearch.TaskStatus
		wantErr    string
	}{
		{name: "succeeded", task: &meilisearch.Task{Status: meilisearch.TaskStatusSucceeded}, wantStatus: meilisearch.TaskStatusSucceeded},
		{name: "failed", task: &meilisearch.Task{Status: meilisearch.TaskStatusFailed}, wantErr: "Meilisearch task 42 failed"},
		{name: "canceled", task: &meilisearch.Task{Status: meilisearch.TaskStatusCanceled}, wantErr: "Meilisearch task 42 canceled"},
		{name: "unexpected status", task: &meilisearch.Task{Status: meilisearch.TaskStatusProcessing}, wantErr: `Meilisearch task 42 returned unexpected status "processing"`},
		{name: "empty task", wantErr: "Meilisearch task 42 returned no result"},
		{name: "wait failure", waitErr: underlyingErr, wantErr: "transport failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := taskReaderStub{wait: func(_ context.Context, gotUID int64, gotInterval time.Duration) (*meilisearch.Task, error) {
				if gotUID != 42 {
					t.Fatalf("task UID=%d, expected 42", gotUID)
				}
				if gotInterval != operationPollInterval {
					t.Fatalf("poll interval=%s, expected %s", gotInterval, operationPollInterval)
				}

				return test.task, test.waitErr
			}}

			got, err := waitTask(context.Background(), reader, 42)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("waitTask() error=%v", err)
				}
				if got == nil || got.Status != test.wantStatus {
					t.Fatalf("waitTask() task=%v, expected status %q", got, test.wantStatus)
				}

				return
			}

			if got != nil && test.name == "empty task" {
				t.Fatalf("waitTask() task=%v, expected nil", got)
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("waitTask() error=%v, expected substring %q", err, test.wantErr)
			}
		})
	}
}

func TestWaitTaskRejectsMissingReader(t *testing.T) {
	_, err := waitTask(context.Background(), nil, 42)
	if err == nil || !strings.Contains(err.Error(), "task manager is not configured") {
		t.Fatalf("waitTask() error=%v, expected missing reader diagnostic", err)
	}
}

func TestWaitTaskIncludesRemoteErrorCode(t *testing.T) {
	var task meilisearch.Task
	if err := json.Unmarshal([]byte(`{"status":"failed","error":{"code":"index_not_found"}}`), &task); err != nil {
		t.Fatalf("decode task fixture: %v", err)
	}

	_, err := waitTask(context.Background(), taskReaderStub{wait: func(context.Context, int64, time.Duration) (*meilisearch.Task, error) {
		return &task, nil
	}}, 42)
	if err == nil || !strings.Contains(err.Error(), "index_not_found") {
		t.Fatalf("waitTask() error=%v, expected remote error code", err)
	}
}

func TestWaitTaskPassesContextToReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := false
	reader := taskReaderStub{wait: func(gotCtx context.Context, taskUID int64, _ time.Duration) (*meilisearch.Task, error) {
		called = true
		if gotCtx != ctx {
			t.Fatal("waitTask did not pass the caller context")
		}
		if taskUID != 7 {
			t.Fatalf("task UID=%d, expected 7", taskUID)
		}

		return &meilisearch.Task{Status: meilisearch.TaskStatusSucceeded}, nil
	}}

	if _, err := waitTask(ctx, reader, 7); err != nil {
		t.Fatalf("waitTask() error=%v", err)
	}
	if !called {
		t.Fatal("waitTask did not call the context-aware waiter")
	}
}
