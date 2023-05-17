package testingHelper

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
	Url         string
	ContentType string
	Content     string
}

type Response struct {
	StatusCode  int
	ContentType string
	Content     string
}

func SendTestRequest(t *testing.T, ts *httptest.Server, args RequestArgs) Response {
	// создаю запрос и устанавливаю contentType
	request, err := http.NewRequest(args.Method, ts.URL+args.Url, strings.NewReader(args.Content))
	request.Header.Set("Content-Type", args.ContentType)
	require.NoError(t, err)
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
	}
}
