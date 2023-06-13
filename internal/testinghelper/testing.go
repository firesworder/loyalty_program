package testinghelper

import (
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type RequestArgs struct {
	Method      string
	URL         string
	ContentType string
	Content     string
	Cookie      *http.Cookie
}

type Response struct {
	StatusCode  int
	ContentType string
	Content     string
	Cookies     []*http.Cookie
}

func SendTestRequest(t *testing.T, ts *httptest.Server, args RequestArgs) Response {
	// создаю запрос и устанавливаю contentType
	request, err := http.NewRequest(args.Method, ts.URL+args.URL, strings.NewReader(args.Content))
	request.Header.Set("Content-Type", args.ContentType)
	require.NoError(t, err)

	// если есть кука (авторизации) - добавить ее
	if args.Cookie != nil {
		request.AddCookie(args.Cookie)
	}

	// отправляю
	respRaw, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	// читаю тело ответа
	defer respRaw.Body.Close()
	content, err := io.ReadAll(respRaw.Body)
	require.NoError(t, err)

	return Response{
		StatusCode:  respRaw.StatusCode,
		ContentType: respRaw.Header.Get("Content-Type"),
		Content:     string(content),
		Cookies:     respRaw.Cookies(),
	}
}
