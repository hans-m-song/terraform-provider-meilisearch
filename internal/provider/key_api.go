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

	"github.com/meilisearch/meilisearch-go"
)

const maxKeyUpdateResponseBytes = 64 * 1024

type keyMetadataUpdate struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type keyAPIError struct {
	status int
	code   string
}

func (e *keyAPIError) Error() string {
	if e.code == "" {
		return fmt.Sprintf("meilisearch API request failed with HTTP status %d", e.status)
	}

	return fmt.Sprintf("meilisearch API request failed with code %q and HTTP status %d", e.code, e.status)
}

func (e *keyAPIError) apiCode() string {
	return e.code
}

func (e *keyAPIError) apiStatus() int {
	return e.status
}

func (r *keyResource) updateKeyWithContext(ctx context.Context, uid string, name, description *string) (*meilisearch.Key, error) {
	if r.provider == nil || r.provider.httpClient == nil || r.provider.host == "" {
		return nil, errors.New("key metadata update client is not configured")
	}

	requestBody := keyMetadataUpdate{
		Name:        name,
		Description: description,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal API key update: %w", err)
	}

	endpoint := strings.TrimRight(r.provider.host, "/") + "/keys/" + url.PathEscape(uid)
	request, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create API key update request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+r.provider.apiKey)

	response, err := r.provider.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send API key update request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxKeyUpdateResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read API key update response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var errorBody struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(responseBody, &errorBody); err != nil {
			errorBody.Code = ""
		}

		return nil, &keyAPIError{status: response.StatusCode, code: errorBody.Code}
	}

	if len(responseBody) == 0 {
		return nil, nil
	}

	var key meilisearch.Key
	if err := json.Unmarshal(responseBody, &key); err != nil {
		return nil, fmt.Errorf("decode API key update response: %w", err)
	}

	return &key, nil
}
