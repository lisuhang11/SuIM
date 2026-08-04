package network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"SuIM/suim-sdk-core/pkg/ccontext"
	"SuIM/suim-sdk-core/pkg/sdkerrs"
)

var apiClient = &http.Client{Timeout: 10 * time.Second}

// ApiResponse matches SuIM apigateway Respond format.
type ApiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func ApiRequest(ctx context.Context, method, path string, reqBody any, resp any) error {
	info := ccontext.Info(ctx)
	if info == nil || info.ApiAddr == "" {
		return sdkerrs.ErrNotInit
	}
	operationID := ccontext.OperationID(ctx)
	if operationID == "" {
		return sdkerrs.Wrap(sdkerrs.ArgsError, "operationID is empty", nil)
	}

	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return sdkerrs.Wrap(sdkerrs.SdkInternalError, "marshal request", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	fullURL := strings.TrimRight(info.ApiAddr, "/") + path
	httpReq, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return sdkerrs.Wrap(sdkerrs.SdkInternalError, "new request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("operationID", operationID)
	if info.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+info.Token)
	}

	httpResp, err := apiClient.Do(httpReq)
	if err != nil {
		return sdkerrs.Wrap(sdkerrs.NetworkError, "http do", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return sdkerrs.Wrap(sdkerrs.SdkInternalError, "read body", err)
	}

	var base ApiResponse
	if err := json.Unmarshal(respBody, &base); err != nil {
		return sdkerrs.Wrap(sdkerrs.SdkInternalError, fmt.Sprintf("parse api response: %s", string(respBody)), err)
	}
	if base.Code != 0 || httpResp.StatusCode >= 400 {
		code := base.Code
		if code == 0 {
			code = httpResp.StatusCode
		}
		msg := base.Message
		if msg == "" {
			msg = httpResp.Status
		}
		return sdkerrs.New(code, msg)
	}
	if resp == nil || len(base.Data) == 0 || string(base.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(base.Data, resp); err != nil {
		return sdkerrs.Wrap(sdkerrs.SdkInternalError, "unmarshal data", err)
	}
	return nil
}

func ApiPost(ctx context.Context, path string, req, resp any) error {
	return ApiRequest(ctx, http.MethodPost, path, req, resp)
}

func ApiGet(ctx context.Context, path string, resp any) error {
	return ApiRequest(ctx, http.MethodGet, path, nil, resp)
}

func ApiPut(ctx context.Context, path string, req, resp any) error {
	return ApiRequest(ctx, http.MethodPut, path, req, resp)
}

func JoinQuery(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}
