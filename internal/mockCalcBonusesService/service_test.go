package mockCalcBonusesService

import (
	"github.com/firesworder/loyalty_program/internal/testingHelper"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMock_handlerGetOrderInfo(t *testing.T) {
	r := NewMock(nil)
	ts := httptest.NewServer(r.Router)
	defer ts.Close()

	tests := []struct {
		name         string
		request      testingHelper.RequestArgs
		wantResponse testingHelper.Response
	}{
		{
			name: "Test 1. Correct request, order number exist + status PROCESSED",
			request: testingHelper.RequestArgs{
				Url:    "/api/orders/4",
				Method: http.MethodGet,
			},
			wantResponse: testingHelper.Response{
				ContentType: "application/json",
				StatusCode:  http.StatusOK,
				Content:     "{\"order\":4,\"status\":\"PROCESSED\",\"accrual\":200}",
			},
		},
		{
			name: "Test 2. Correct request, order number exist + status not PROCESSED",
			request: testingHelper.RequestArgs{
				Url:    "/api/orders/3",
				Method: http.MethodGet,
			},
			wantResponse: testingHelper.Response{
				ContentType: "application/json",
				StatusCode:  http.StatusOK,
				Content:     "{\"order\":3,\"status\":\"PROCESSING\"}",
			},
		},
		{
			name: "Test 3. Correct request, order number not exist",
			request: testingHelper.RequestArgs{
				Url:    "/api/orders/100",
				Method: http.MethodGet,
			},
			wantResponse: testingHelper.Response{
				ContentType: "text/plain",
				StatusCode:  http.StatusNoContent,
				Content:     "",
			},
		},
		// requests with errors
		{
			name: "Test 4. Incorrect request, order number not parseable to int",
			request: testingHelper.RequestArgs{
				Url:    "/api/orders/AB100",
				Method: http.MethodGet,
			},
			wantResponse: testingHelper.Response{
				ContentType: "text/plain",
				StatusCode:  http.StatusNoContent,
				Content:     "",
			},
		},
		{
			name: "Test 5. Incorrect request, wrong http method",
			request: testingHelper.RequestArgs{
				Url:    "/api/orders/AB100",
				Method: http.MethodPost,
			},
			wantResponse: testingHelper.Response{
				StatusCode:  http.StatusMethodNotAllowed,
				ContentType: "",
				Content:     "",
			},
		},
		{
			name: "Test 6. Incorrect request, wrong url",
			request: testingHelper.RequestArgs{
				Url:    "/api/order/100",
				Method: http.MethodPost,
			},
			wantResponse: testingHelper.Response{
				ContentType: "text/plain; charset=utf-8",
				StatusCode:  http.StatusNotFound,
				Content:     "404 page not found\n",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResponse := testingHelper.SendTestRequest(t, ts, tt.request)
			assert.Equal(t, tt.wantResponse, gotResponse)
		})
	}
}

// TestNewMock объединяет в себе тестирование и initRouter и InitMockData
func TestNewMock(t *testing.T) {
	defaUltMockData := map[int]Bonus{
		1: {OrderId: 1, Status: REGISTERED},
		2: {OrderId: 2, Status: INVALID},
		3: {OrderId: 3, Status: PROCESSING},
		4: {OrderId: 4, Status: PROCESSED, Accrual: 200},
		5: {OrderId: 5, Status: PROCESSED, Accrual: 500},
	}
	customMockData := map[int]Bonus{
		10: {OrderId: 10, Status: PROCESSED, Accrual: 1000},
		15: {OrderId: 15, Status: REGISTERED},
	}
	type args struct {
		mockData map[int]Bonus
	}
	tests := []struct {
		name string
		args args
		want *Mock
	}{
		{
			name: "Test 1. Default(nil args) params.",
			args: args{mockData: nil},
			want: &Mock{mockData: defaUltMockData},
		},
		{
			name: "Test 2. Custom mock data.",
			args: args{mockData: customMockData},
			want: &Mock{mockData: customMockData},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMockService := NewMock(tt.args.mockData)
			assert.NotNil(t, gotMockService.Router)
			assert.Equal(t, gotMockService.mockData, tt.want.mockData)
		})
	}
}
