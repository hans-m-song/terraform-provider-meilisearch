package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type indexSettingsTask struct {
	TaskUID *int64 `json:"taskUid"`
}

type indexSettingsAPIError struct {
	status int
	code   string
}

type indexSettingsAcceptedError struct {
	err error
}

func (e *indexSettingsAcceptedError) Error() string {
	return e.err.Error()
}

func (e *indexSettingsAcceptedError) Unwrap() error {
	return e.err
}

func indexSettingsResponseAccepted(err error) bool {
	var accepted *indexSettingsAcceptedError
	return errors.As(err, &accepted)
}

func indexSettingsErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var accepted *indexSettingsAcceptedError
	if errors.As(err, &accepted) {
		return accepted.Error()
	}
	var coded codedAPIError
	if errors.As(err, &coded) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return apiError(err)
	}
	return "Meilisearch operation failed"
}

func (e *indexSettingsAPIError) Error() string {
	if e.code == "" {
		return fmt.Sprintf("meilisearch API request failed with HTTP status %d", e.status)
	}

	return fmt.Sprintf("meilisearch API request failed with code %q and HTTP status %d", e.code, e.status)
}

func (e *indexSettingsAPIError) apiCode() string {
	return e.code
}

func (e *indexSettingsAPIError) apiStatus() int {
	return e.status
}

func (r *indexSettingsResource) getSettings(ctx context.Context, uid string) (indexSettingsValues, error) {
	if r.provider == nil || r.provider.host == "" {
		return indexSettingsValues{}, errors.New("index settings HTTP client is not configured")
	}

	body, err := r.doSettingsRequest(ctx, http.MethodGet, uid, nil)
	if err != nil {
		return indexSettingsValues{}, err
	}

	return decodeIndexSettings(body)
}

func (r *indexSettingsResource) patchSettings(ctx context.Context, uid string, patch map[string]json.RawMessage) (*indexSettingsTask, error) {
	if r.provider == nil || r.provider.host == "" {
		return nil, errors.New("index settings HTTP client is not configured")
	}

	body, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshal index settings update: %w", err)
	}

	responseBody, err := r.doSettingsRequest(ctx, http.MethodPatch, uid, body)
	if err != nil {
		return nil, err
	}

	var task indexSettingsTask
	if err := json.Unmarshal(responseBody, &task); err != nil {
		return nil, &indexSettingsAcceptedError{err: errors.New("meilisearch returned an invalid settings task response")}
	}
	if task.TaskUID == nil {
		return nil, &indexSettingsAcceptedError{err: errors.New("meilisearch returned no settings task UID")}
	}

	return &task, nil
}

func (r *indexSettingsResource) doSettingsRequest(ctx context.Context, method, uid string, body []byte) ([]byte, error) {
	httpClient := r.provider.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	endpoint := strings.TrimRight(r.provider.host, "/") + "/indexes/" + url.PathEscape(uid) + "/settings"
	request, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create index settings request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if method == http.MethodPatch {
		request.Header.Set("Content-Type", "application/json")
	}
	if r.provider.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+r.provider.apiKey)
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send index settings request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	limited := io.LimitReader(response.Body, maxIndexSettingsResponseBytes+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		if method == http.MethodPatch && response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			return nil, &indexSettingsAcceptedError{err: errors.New("could not read the accepted settings task response")}
		}
		return nil, fmt.Errorf("read index settings response: %w", err)
	}
	if len(responseBody) > maxIndexSettingsResponseBytes {
		if method == http.MethodPatch && response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			return nil, &indexSettingsAcceptedError{err: errors.New("the accepted settings task response was oversized")}
		}
		return nil, errors.New("meilisearch returned an oversized index settings response")
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var errorBody struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(responseBody, &errorBody); err != nil {
			errorBody.Code = ""
		}
		return nil, &indexSettingsAPIError{status: response.StatusCode, code: errorBody.Code}
	}

	return responseBody, nil
}

func isMissingIndexError(err error) bool {
	return isAPIError(err, "index_not_found")
}
