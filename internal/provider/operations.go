package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

const defaultOperationTimeout = 5 * time.Minute

const operationPollInterval = 500 * time.Millisecond

func operationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultOperationTimeout
	}

	return context.WithTimeout(ctx, timeout)
}

func apiError(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return contextError(err)
	}

	var taskError *taskOperationError
	if errors.As(err, &taskError) {
		return taskError.Error()
	}

	var keyError *keyAPIError
	if errors.As(err, &keyError) {
		if keyError.code != "" && keyError.status > 0 {
			return fmt.Sprintf("Meilisearch API error %q (HTTP status %d)", keyError.code, keyError.status)
		}
		if keyError.status > 0 {
			return fmt.Sprintf("Meilisearch API error (HTTP status %d)", keyError.status)
		}
		return "Meilisearch API error"
	}

	var meilisearchError *meilisearch.Error
	if errors.As(err, &meilisearchError) {
		if meilisearchError.OriginError != nil {
			if errors.Is(meilisearchError.OriginError, context.DeadlineExceeded) || errors.Is(meilisearchError.OriginError, context.Canceled) {
				return contextError(meilisearchError.OriginError)
			}
		}

		code := meilisearchError.MeilisearchApiError.Code
		status := meilisearchError.StatusCode

		switch {
		case code != "" && status > 0:
			return fmt.Sprintf("Meilisearch API error %q (HTTP status %d)", code, status)
		case code != "":
			return fmt.Sprintf("Meilisearch API error %q", code)
		case status > 0:
			return fmt.Sprintf("Meilisearch API error (HTTP status %d)", status)
		default:
			return "Meilisearch API error"
		}
	}

	return "Meilisearch operation failed"
}

func contextError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "operation deadline exceeded"
	}

	return "operation canceled"
}

func isAPIError(err error, code string) bool {
	if err == nil {
		return false
	}

	var meilisearchError *meilisearch.Error
	if errors.As(err, &meilisearchError) {
		return meilisearchError.MeilisearchApiError.Code == code
	}

	var taskError *taskOperationError
	if errors.As(err, &taskError) {
		return taskError.code == code
	}

	var keyError *keyAPIError
	return errors.As(err, &keyError) && keyError.apiCode() == code
}

func waitTask(ctx context.Context, taskReader meilisearch.TaskReader, taskUID int64) (*meilisearch.Task, error) {
	if taskReader == nil {
		return nil, errors.New("meilisearch task manager is not configured")
	}

	task, err := taskReader.WaitForTaskWithContext(ctx, taskUID, operationPollInterval)
	if err != nil {
		return task, err
	}

	if task == nil {
		return nil, &taskOperationError{uid: taskUID, noResult: true}
	}

	switch task.Status {
	case meilisearch.TaskStatusSucceeded:
		return task, nil
	case meilisearch.TaskStatusFailed, meilisearch.TaskStatusCanceled:
		return task, &taskOperationError{uid: taskUID, status: task.Status, code: task.Error.Code}
	default:
		return task, &taskOperationError{uid: taskUID, status: task.Status, unexpected: true}
	}
}

type taskOperationError struct {
	uid        int64
	status     meilisearch.TaskStatus
	code       string
	noResult   bool
	unexpected bool
}

func (e *taskOperationError) Error() string {
	if e.noResult {
		return fmt.Sprintf("Meilisearch task %d returned no result", e.uid)
	}
	if e.unexpected {
		return fmt.Sprintf("Meilisearch task %d returned unexpected status %q", e.uid, e.status)
	}
	if e.code == "" {
		return fmt.Sprintf("Meilisearch task %d %s", e.uid, e.status)
	}

	return fmt.Sprintf("Meilisearch task %d %s (code %q)", e.uid, e.status, e.code)
}
