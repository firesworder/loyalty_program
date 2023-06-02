package updatingWorker

import (
	"encoding/json"
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"
)

func getMockHandler(t *testing.T) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		orderNumberReq := path.Base(request.RequestURI)

		if len(orderNumberReq) < 5 {
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		} else if len(orderNumberReq) < 10 {
			writer.WriteHeader(http.StatusNoContent)
			return
		} else {
			r := struct {
				Order  string `json:"order"`
				Status string `json:"status"`
				Amount int64  `json:"accrual"`
			}{
				Order:  orderNumberReq,
				Status: "PROCESSED",
				Amount: 500,
			}

			rJson, err := json.Marshal(r)
			require.NoError(t, err)

			writer.WriteHeader(http.StatusOK)
			writer.Write(rJson)
		}
	}
}

func TestWorker_sendOrderStatusRequest(t *testing.T) {
	s := storage.NewMock()
	w := NewWorker(s, time.Minute)

	ts := httptest.NewServer(getMockHandler(t))
	defer ts.Close()

	w.ApiCalcAddress = ts.URL + "/api/orders/"

	tests := []struct {
		name        string
		orderNumber string
		wantErr     error
		wantRB      responseBody
	}{
		{
			name:        "Test 1. Correct(200) request.",
			orderNumber: "328257446760",
			wantErr:     nil,
			wantRB: responseBody{
				Order:  "328257446760",
				Status: "PROCESSED",
				Amount: 500,
			},
		},
		{
			name:        "Test 2. Incorrect(204) request. Order number not found",
			orderNumber: "12345678",
			wantErr:     ErrOrderNotFound,
			wantRB:      responseBody{},
		},
		{
			name:        "Test 3. Incorrect request. Too many request",
			orderNumber: "1234",
			wantErr:     ErrReqLimitExceeded,
			wantRB:      responseBody{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rB, err := w.sendOrderStatusRequest(tt.orderNumber)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantRB, rB)
		})
	}
}

func TestWorker_Start(t *testing.T) {
	ts := httptest.NewServer(getMockHandler(t))
	defer ts.Close()

	s := storage.NewMock()
	w := NewWorker(s, 1*time.Second)
	w.ApiCalcAddress = ts.URL + "/api/orders/"

	// демо данные
	s.OrderStatus = []storage.MockOrderStatus{
		storage.MockOrderStatusData[5],
		storage.MockOrderStatusData[4],
		storage.MockOrderStatusData[3],
		storage.MockOrderStatusData[1],
	}
	s.OrderStatus[3].Number = "636197079784"

	// желаемый результат
	wantOS := make([]storage.MockOrderStatus, len(s.OrderStatus))
	copy(wantOS, s.OrderStatus)
	// единственный заказ который должен измениться!
	wantOS[1].Amount, wantOS[3].Amount = 500, 500
	wantOS[1].Status, wantOS[3].Status = "PROCESSED", "PROCESSED"

	// запуск тестируемой функции
	go w.Start()
	time.Sleep(1500 * time.Millisecond)
	w.done <- struct{}{}

	assert.Equal(t, wantOS, s.OrderStatus)
}
