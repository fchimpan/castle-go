package castle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"

	"github.com/fchimpan/castle-go/api"
)

const defaultMaxResponseBytes = 4 << 20

type responsePolicy struct {
	status       int
	contentTypes []string
	assessment   bool
}

func finishResponse[T any](resp *http.Response, requestErr error, parse func(*http.Response) (*T, error), maxResponseBytes int64, policies ...responsePolicy) (*T, error) {
	if requestErr != nil {
		return nil, safeError("API request failed", requestErr)
	}
	defer resp.Body.Close()
	index := slices.IndexFunc(policies, func(policy responsePolicy) bool { return policy.status == resp.StatusCode })
	if index < 0 {
		return nil, &APIError{StatusCode: resp.StatusCode}
	}
	policy := policies[index]
	var data []byte
	var contentType string
	if len(policy.contentTypes) > 0 {
		var err error
		data, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		if err != nil {
			return nil, safeError("could not read API response", err)
		}
		if int64(len(data)) > maxResponseBytes {
			return nil, safeError("API response exceeds size limit", nil)
		}
		contentType, _, err = mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil || !slices.Contains(policy.contentTypes, contentType) {
			return nil, safeError("unexpected API response content type", nil)
		}
		if contentType == "application/json" {
			if !json.Valid(data) || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
				return nil, safeError("invalid API response", nil)
			}
			if policy.assessment && !validAssessment(data) {
				return nil, safeError("invalid assessment response", nil)
			}
		}
	}
	// Parsers own their input body. Keep transport cleanup separate from this bounded copy.
	parsedResponse := new(*resp)
	parsedResponse.Body = io.NopCloser(bytes.NewReader(data))
	parsedResponse.Header = resp.Header.Clone()
	if contentType != "" {
		parsedResponse.Header.Set("Content-Type", contentType)
	}
	result, err := parse(parsedResponse)
	parsedResponse.Header = resp.Header
	if err != nil {
		return nil, safeError("invalid API response", err)
	}
	return result, nil
}

func validAssessment(data []byte) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return false
	}
	for _, key := range []string{"policy", "signals", "device"} {
		var object map[string]json.RawMessage
		if json.Unmarshal(fields[key], &object) != nil || object == nil {
			return false
		}
	}
	var result api.FilterAndRiskResponse
	if json.Unmarshal(data, &result) != nil {
		return false
	}
	_, err := result.ValueByDiscriminator()
	return err == nil
}

func safeError(stage string, cause error) error {
	var sentinel error
	if errors.Is(cause, context.Canceled) {
		sentinel = context.Canceled
	} else if errors.Is(cause, context.DeadlineExceeded) {
		sentinel = context.DeadlineExceeded
	}
	return &stageError{stage: stage, cause: sentinel}
}
