package server

import (
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/firesworder/loyalty_program/internal/testingHelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testCookie struct {
	name  string
	value string
}

func getCookie(cookies []*http.Cookie, cookieName string) *testCookie {
	for _, cookie := range cookies {
		if cookie.Name == cookieName {
			return &testCookie{name: cookie.Name, value: cookie.Value}
		}
	}
	return nil
}

func Test_checkReqAuthData(t *testing.T) {
	gotUser := &postArgsUser{}
	ts := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotUser = checkReqAuthData(writer, request)
	}))
	defer ts.Close()

	tests := []struct {
		name     string
		req      testingHelper.RequestArgs
		wantResp testingHelper.Response
		wantUser *postArgsUser
	}{
		{
			name: "Test 1. Correct auth request data",
			req: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				ContentType: "application/json",
				Content:     `{"login": "admin", "password": "admin"}`,
			},
			wantResp: testingHelper.Response{
				StatusCode:  http.StatusOK,
				ContentType: "",
				Content:     "",
				Cookies:     []*http.Cookie{},
			},
			wantUser: &postArgsUser{Login: "admin", Password: "admin"},
		},
		{
			name: "Test 2. Empty or not set login",
			req: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				ContentType: "application/json",
				Content:     `{"password": "admin"}`,
			},
			wantResp: testingHelper.Response{
				StatusCode:  http.StatusBadRequest,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login and passwords fields can not be empty\n",
				Cookies:     []*http.Cookie{},
			},
			wantUser: nil,
		},
		{
			name: "Test 3. Empty or not set password",
			req: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				ContentType: "application/json",
				Content:     `{"login": "admin", "password": ""}`,
			},
			wantResp: testingHelper.Response{
				StatusCode:  http.StatusBadRequest,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login and passwords fields can not be empty\n",
				Cookies:     []*http.Cookie{},
			},
			wantUser: nil,
		},
		{
			name: "Test 4. Empty login and password",
			req: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				ContentType: "application/json",
				Content:     `{"login": "", "password": ""}`,
			},
			wantResp: testingHelper.Response{
				StatusCode:  http.StatusBadRequest,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login and passwords fields can not be empty\n",
				Cookies:     []*http.Cookie{},
			},
			wantUser: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUser = &postArgsUser{}
			gotResp := testingHelper.SendTestRequest(t, ts, tt.req)
			assert.Equal(t, tt.wantResp, gotResp)
			assert.Equal(t, tt.wantUser, gotUser)
		})
	}
}

func Test_setAuthTokenCookie(t *testing.T) {
	cookieName, cookieValue := "token", "some_token"
	ts := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		setTokenCookie(writer, cookieValue)
	}))
	defer ts.Close()

	resp := testingHelper.SendTestRequest(t, ts, testingHelper.RequestArgs{})

	gotC := getCookie(resp.Cookies, cookieName)
	require.NotNil(t, gotC)
	assert.Equal(t, gotC.value, cookieValue)
}

func TestServer_handlerLoginUser(t *testing.T) {
	serverObj := NewServer("", storage.NewMock())
	ts := httptest.NewServer(serverObj.Router)
	defer ts.Close()

	tests := []struct {
		name         string
		reqArgs      testingHelper.RequestArgs
		wantResponse testingHelper.Response
		wantCookie   bool
	}{
		{
			name: "Test 1. Correct auth data.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/login",
				ContentType: "application/json",
				Content:     `{"login": "admin", "password": "admin"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusOK,
				ContentType: "",
				Content:     "",
			},
			wantCookie: true,
		},
		{
			name: "Test 2. Incorrect auth data. Incorrect password.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/login",
				ContentType: "application/json",
				Content:     `{"login": "admin", "password": "postgres"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusUnauthorized,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login or password incorrect\n",
			},
			wantCookie: false,
		},
		{
			name: "Test 3. Incorrect auth data. User don't exist.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/login",
				ContentType: "application/json",
				Content:     `{"login": "randomLogin", "password": "postgres"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusUnauthorized,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login or password incorrect\n",
			},
			wantCookie: false,
		},
		{
			name: "Test 4. Incorrect http method",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPut,
				Url:         "/api/user/login",
				ContentType: "application/json",
				Content:     `{"login": "randomLogin", "password": "postgres"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusMethodNotAllowed,
				ContentType: "",
				Content:     "",
			},
			wantCookie: false,
		},
		{
			name: "Test 5. Incorrect request body. Not set password field.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/login",
				ContentType: "application/json",
				Content:     `{"login": "admin"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusBadRequest,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login and passwords fields can not be empty\n",
			},
			wantCookie: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResp := testingHelper.SendTestRequest(t, ts, tt.reqArgs)
			// проверяем куку
			gotCookie := getCookie(gotResp.Cookies, TokenCookieName)
			assert.Equal(t, tt.wantCookie, gotCookie != nil)
			if tt.wantCookie {
				// проверяем наличие пользователя в кеше
				_, userInCache := serverObj.TokensCache.Users[gotCookie.value]
				assert.Equal(t, true, userInCache)
			}
			// проверяем ответ
			gotResp.Cookies = nil
			assert.Equal(t, tt.wantResponse, gotResp)
		})
	}
}

func TestServer_handlerRegisterUser(t *testing.T) {
	userAdmin := &storage.MockUser{Login: "admin", HashedPassword: "admin"}
	userPostgres := &storage.MockUser{Login: "postgres", HashedPassword: "postgres"}

	serverObj := NewServer("", storage.NewMock())
	ts := httptest.NewServer(serverObj.Router)
	defer ts.Close()

	tests := []struct {
		name            string
		reqArgs         testingHelper.RequestArgs
		wantResponse    testingHelper.Response
		wantCookie      bool
		wantUserStorage []storage.User
	}{
		{
			name: "Test 1. Correct reg data.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/register",
				ContentType: "application/json",
				Content:     `{"login": "mysql", "password": "mysql"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusOK,
				ContentType: "",
				Content:     "",
			},
			wantCookie: true,
			wantUserStorage: []storage.User{
				userAdmin,
				userPostgres,
				&storage.MockUser{Login: "mysql", HashedPassword: "mysql"},
			},
		},
		{
			name: "Test 2. Incorrect reg data. Login already exist.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/register",
				ContentType: "application/json",
				Content:     `{"login": "postgres", "password": "postgres"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusConflict,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login already exist\n",
			},
			wantCookie:      false,
			wantUserStorage: []storage.User{userAdmin, userPostgres},
		},
		{
			name: "Test 3. Incorrect http method",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPut,
				Url:         "/api/user/register",
				ContentType: "application/json",
				Content:     `{"login": "randomLogin", "password": "postgres"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusMethodNotAllowed,
				ContentType: "",
				Content:     "",
			},
			wantCookie:      false,
			wantUserStorage: []storage.User{userAdmin, userPostgres},
		},
		{
			name: "Test 4. Incorrect request body. Not set password field.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/register",
				ContentType: "application/json",
				Content:     `{"login": "admin"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusBadRequest,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login and passwords fields can not be empty\n",
			},
			wantCookie:      false,
			wantUserStorage: []storage.User{userAdmin, userPostgres},
		},
		{
			name: "Test 5. Incorrect request body. Not set login field.",
			reqArgs: testingHelper.RequestArgs{
				Method:      http.MethodPost,
				Url:         "/api/user/register",
				ContentType: "application/json",
				Content:     `{"password": "admin"}`,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusBadRequest,
				ContentType: "text/plain; charset=utf-8",
				Content:     "login and passwords fields can not be empty\n",
			},
			wantCookie:      false,
			wantUserStorage: []storage.User{userAdmin, userPostgres},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResp := testingHelper.SendTestRequest(t, ts, tt.reqArgs)
			// проверяем куку
			gotCookie := getCookie(gotResp.Cookies, TokenCookieName)
			assert.Equal(t, tt.wantCookie, gotCookie != nil)
			if tt.wantCookie {
				// проверяем наличие пользователя в кеше
				_, userInCache := serverObj.TokensCache.Users[gotCookie.value]
				assert.Equal(t, true, userInCache)
			}
			// проверяем ответ
			gotResp.Cookies = nil
			assert.Equal(t, tt.wantResponse, gotResp)
		})
	}
}
