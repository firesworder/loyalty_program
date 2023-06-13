package server

import (
	"fmt"
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/firesworder/loyalty_program/internal/testinghelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_checkOrderNumberByLuhn(t *testing.T) {
	tests := []struct {
		name        string
		orderNumber string
		wantErr     error
	}{
		{
			name:        "Test 1. Correct orderNumber.",
			orderNumber: "456951314651",
			wantErr:     nil,
		},
		{
			name:        "Test 2. Incorrect orderNumber. Contains letters and spec symbols.",
			orderNumber: "456951b314651@a",
			wantErr:     fmt.Errorf("order number can only contains digits"),
		},
		{
			name:        "Test 3. Incorrect orderNumber. Control number(last digit) differs.",
			orderNumber: "456951314658",
			wantErr:     fmt.Errorf("order number incorrect"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkOrderNumberByLuhn(tt.orderNumber)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestServer_handlerRegisterOrderNumber(t *testing.T) {
	sM := storage.NewMock()
	server, err := NewServer("", sM)
	require.NoError(t, err)
	server.TokensCache.Users["token"] = storage.User{Login: "admin", Password: "admin"}
	ts := httptest.NewServer(server.Router)
	defer ts.Close()

	tests := []struct {
		name         string
		reqArgs      testinghelper.RequestArgs
		wantResponse testinghelper.Response
	}{
		{
			name: "Test 1. Correct request.",
			reqArgs: testinghelper.RequestArgs{
				Method:      http.MethodPost,
				URL:         "/api/user/orders",
				ContentType: ContentTypeJSON,
				Content:     "4561261212345467",
				Cookie:      &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode: http.StatusAccepted,
			},
		},
		{
			name: "Test 2. Order already registered by that user.",
			reqArgs: testinghelper.RequestArgs{
				Method:      http.MethodPost,
				URL:         "/api/user/orders",
				ContentType: ContentTypeJSON,
				Content:     "9359943520",
				Cookie:      &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode: http.StatusOK,
			},
		},
		{
			name: "Test 3. Incorrect request. Empty body.",
			reqArgs: testinghelper.RequestArgs{
				Method: http.MethodPost,
				URL:    "/api/user/orders",
				Cookie: &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusBadRequest,
				ContentType: "text/plain; charset=utf-8",
				Content:     "body can not be empty\n",
				Cookies:     nil,
			},
		},
		{
			name: "Test 4. Order has been registered already by another user.",
			reqArgs: testinghelper.RequestArgs{
				Method:      http.MethodPost,
				URL:         "/api/user/orders",
				ContentType: ContentTypeJSON,
				Content:     "328257446760",
				Cookie:      &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusConflict,
				ContentType: "text/plain; charset=utf-8",
				Content:     "order has been registered already by other user\n",
				Cookies:     nil,
			},
		},
		{
			name: "Test 5. Incorrect order number.",
			reqArgs: testinghelper.RequestArgs{
				Method:      http.MethodPost,
				URL:         "/api/user/orders",
				ContentType: ContentTypeJSON,
				Content:     "328257446767",
				Cookie:      &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusUnprocessableEntity,
				ContentType: "text/plain; charset=utf-8",
				Content:     "order number incorrect\n",
				Cookies:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sM.ResetData()
			gotResp := testinghelper.SendTestRequest(t, ts, tt.reqArgs)
			gotResp.Cookies = nil
			assert.Equal(t, tt.wantResponse, gotResp)
		})
	}
}

func TestServer_handlerGetOrderStatusList(t *testing.T) {
	server, err := NewServer("", storage.NewMock())
	require.NoError(t, err)
	server.TokensCache.Users["token"] = storage.User{Login: "admin", Password: "admin"}
	ts := httptest.NewServer(server.Router)
	defer ts.Close()

	tests := []struct {
		name         string
		reqArgs      testinghelper.RequestArgs
		wantResponse testinghelper.Response
	}{
		{
			name: "Test 1. Correct request.",
			reqArgs: testinghelper.RequestArgs{
				Method: http.MethodGet,
				URL:    "/api/user/orders",
				Cookie: &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusOK,
				ContentType: ContentTypeJSON,
				Content: `[{"number":"order1","status":"PROCESSED","accrual":100,"uploaded_at":"2022-12-10T12:00:00+04:00"},` +
					`{"number":"order3","status":"INVALID","uploaded_at":"2023-02-10T12:00:00+04:00"},` +
					`{"number":"9359943520","status":"NEW","uploaded_at":"2023-03-10T12:00:00+04:00"}]`,
				Cookies: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResp := testinghelper.SendTestRequest(t, ts, tt.reqArgs)
			gotResp.Cookies = nil
			assert.Equal(t, tt.wantResponse, gotResp)
		})
	}
}

func TestServer_handlerGetBalance(t *testing.T) {
	server, err := NewServer("", storage.NewMock())
	require.NoError(t, err)
	server.TokensCache.Users["token"] = storage.User{Login: "admin", Password: "admin"}
	ts := httptest.NewServer(server.Router)
	defer ts.Close()

	tests := []struct {
		name         string
		reqArgs      testinghelper.RequestArgs
		wantResponse testinghelper.Response
	}{
		{
			name: "Test 1. Correct request.",
			reqArgs: testinghelper.RequestArgs{
				Method: http.MethodGet,
				URL:    "/api/user/balance",
				Cookie: &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusOK,
				ContentType: ContentTypeJSON,
				Content:     `{"current":900,"withdrawn":15}`,
				Cookies:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResp := testinghelper.SendTestRequest(t, ts, tt.reqArgs)
			gotResp.Cookies = nil
			assert.Equal(t, tt.wantResponse, gotResp)
		})
	}
}

func TestServer_handlerWithdrawBonuses(t *testing.T) {
	sM := storage.NewMock()
	server, err := NewServer("", sM)
	require.NoError(t, err)
	server.TokensCache.Users["token"] = storage.User{Login: "admin", Password: "admin"}
	ts := httptest.NewServer(server.Router)
	defer ts.Close()

	tests := []struct {
		name         string
		reqArgs      testinghelper.RequestArgs
		wantResponse testinghelper.Response
	}{
		{
			name: "Test 1. Correct request.",
			reqArgs: testinghelper.RequestArgs{
				Method:      http.MethodPost,
				URL:         "/api/user/balance/withdraw",
				ContentType: ContentTypeJSON,
				Content:     `{"order": "456951314651", "sum": 100}`,
				Cookie:      &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode: http.StatusOK,
			},
		},
		{
			name: "Test 2. Balance exceeded.",
			reqArgs: testinghelper.RequestArgs{
				Method:      http.MethodPost,
				URL:         "/api/user/balance/withdraw",
				ContentType: ContentTypeJSON,
				Content:     `{"order": "456951314651", "sum": 1000}`,
				Cookie:      &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusPaymentRequired,
				ContentType: "text/plain; charset=utf-8",
				Content:     "balance exceeded\n",
			},
		},
		{
			name: "Test 3. Incorrect request. Empty body.",
			reqArgs: testinghelper.RequestArgs{
				Method: http.MethodPost,
				URL:    "/api/user/balance/withdraw",
				Cookie: &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusInternalServerError,
				ContentType: "text/plain; charset=utf-8",
				Content:     "\n",
				Cookies:     nil,
			},
		},
		{
			name: "Test 4. Incorrect order number.",
			reqArgs: testinghelper.RequestArgs{
				Method:      http.MethodPost,
				URL:         "/api/user/balance/withdraw",
				ContentType: ContentTypeJSON,
				Content:     `{"order": "456951314655", "sum": 100}`,
				Cookie:      &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusUnprocessableEntity,
				ContentType: "text/plain; charset=utf-8",
				Content:     "order number incorrect\n",
				Cookies:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sM.ResetData()
			gotResp := testinghelper.SendTestRequest(t, ts, tt.reqArgs)
			gotResp.Cookies = nil
			assert.Equal(t, tt.wantResponse, gotResp)
		})
	}
}

func TestServer_handlerGetWithdrawals(t *testing.T) {
	server, err := NewServer("", storage.NewMock())
	require.NoError(t, err)
	server.TokensCache.Users["token"] = storage.User{Login: "admin", Password: "admin"}
	ts := httptest.NewServer(server.Router)
	defer ts.Close()

	tests := []struct {
		name         string
		reqArgs      testinghelper.RequestArgs
		wantResponse testinghelper.Response
	}{
		{
			name: "Test 1. Correct request.",
			reqArgs: testinghelper.RequestArgs{
				Method: http.MethodGet,
				URL:    "/api/user/withdrawals",
				Cookie: &http.Cookie{Name: TokenCookieName, Value: "token", Expires: time.Now().Add(TokenExpires)},
			},
			wantResponse: testinghelper.Response{
				StatusCode:  http.StatusOK,
				ContentType: ContentTypeJSON,
				Content:     `[{"order":"order6","sum":100,"processed_at":"2023-04-02T12:00:00+04:00"}]`,
				Cookies:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResp := testinghelper.SendTestRequest(t, ts, tt.reqArgs)
			gotResp.Cookies = nil
			assert.Equal(t, tt.wantResponse, gotResp)
		})
	}
}
