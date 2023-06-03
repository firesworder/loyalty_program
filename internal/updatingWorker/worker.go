package updatingWorker

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

const calcApi = "/api/orders/"

type Worker struct {
	Storage           storage.Storage
	ReqPerMinuteLimit int64
	ApiCalcAddress    string
	TickerDuration    time.Duration
	done              chan struct{}
}

func NewWorker(storage storage.Storage, tickerDuration time.Duration) *Worker {
	w := &Worker{Storage: storage, ApiCalcAddress: calcApi, TickerDuration: tickerDuration}
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
	Order  string `json:"order"`
	Status string `json:"status"`
	Amount int64  `json:"accrual"`
}

func (w *Worker) sendOrderStatusRequest(orderNumber string) (rB responseBody, err error) {
	request, err := http.NewRequest(http.MethodGet, w.ApiCalcAddress+orderNumber, nil)
	if err != nil {
		return
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return
	}

	if response.StatusCode == http.StatusTooManyRequests {
		return rB, ErrReqLimitExceeded
	}
	if response.StatusCode == http.StatusNoContent {
		return rB, ErrOrderNotFound
	}

	err = json.NewDecoder(response.Body).Decode(&rB)
	return
}
