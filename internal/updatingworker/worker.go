package updatingworker

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/firesworder/loyalty_program/internal/storage"
	"log"
	"net/http"
	"time"
)

var ErrReqLimitExceeded = errors.New("too many request, cancel current queue")
var ErrOrderNotFound = errors.New("order number was not found")

type Worker struct {
	Storage           storage.Storage
	ReqPerMinuteLimit int64
	APICalcAddress    string
	TickerDuration    time.Duration
	done              chan struct{}
}

func NewWorker(storage storage.Storage, tickerDuration time.Duration, apiCalcAddr string) *Worker {
	w := &Worker{Storage: storage, APICalcAddress: apiCalcAddr, TickerDuration: tickerDuration}
	w.done = make(chan struct{})
	return w
}

func (w *Worker) Start() {
	ticker := time.NewTicker(w.TickerDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.updateTick()
		case <-w.done:
			return
		}
	}
}

func (w *Worker) updateTick() {
	oSList, err := w.Storage.GetOrdersWithTemporaryStatus(context.Background())
	// если произошла ошибка на этапе получения заказов - ждать следующей итерации
	if err != nil {
		log.Println(err)
		return
	}

	// опрашиваем сервис по каждому из заказов
	updatedOrderStatuses := make([]storage.OrderStatus, 0)
	for _, order := range oSList {
		rB, err := w.sendOrderStatusRequest(order.Number)
		if err != nil {
			log.Println(err)
			if errors.Is(err, ErrReqLimitExceeded) {
				break
			}
			continue
		}
		updatedOrderStatuses = append(updatedOrderStatuses, storage.OrderStatus{
			Number: rB.Order,
			Status: rB.Status,
			Amount: rB.Amount,
			UserID: order.UserID,
		})
	}

	// пакетное обновление статусов
	err = w.Storage.UpdateOrderStatuses(context.Background(), updatedOrderStatuses)
	if err != nil {
		log.Println(err)
		return
	}
}

type responseBody struct {
	Order  string  `json:"order"`
	Status string  `json:"status"`
	Amount float64 `json:"accrual"`
}

func (w *Worker) sendOrderStatusRequest(orderNumber string) (rB responseBody, err error) {
	url := "/api/orders/" + orderNumber
	request, err := http.NewRequest(http.MethodGet, w.APICalcAddress+url, nil)
	if err != nil {
		return
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusTooManyRequests {
		return rB, ErrReqLimitExceeded
	}
	if response.StatusCode == http.StatusNoContent {
		return rB, ErrOrderNotFound
	}

	err = json.NewDecoder(response.Body).Decode(&rB)
	return
}
