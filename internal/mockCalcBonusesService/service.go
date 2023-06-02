package mockCalcBonusesService

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
)

// todo: удалить

type Mock struct {
	Router   chi.Router
	mockData map[int]Bonus
}

func NewMock(mockData map[int]Bonus) *Mock {
	m := Mock{}
	m.InitRouter()

	m.mockData = mockData
	if mockData == nil {
		m.InitMockData()
	}
	return &m
}

func (m *Mock) InitRouter() {
	m.Router = chi.NewRouter()
	m.Router.Get("/api/orders/{number}", m.handlerGetOrderInfo)
}

func (m *Mock) InitMockData() {
	m.mockData = map[int]Bonus{
		1: {OrderId: 1, Status: REGISTERED},
		2: {OrderId: 2, Status: INVALID},
		3: {OrderId: 3, Status: PROCESSING},
		4: {OrderId: 4, Status: PROCESSED, Accrual: 200},
		5: {OrderId: 5, Status: PROCESSED, Accrual: 500},
	}
}

func (m *Mock) handlerGetOrderInfo(writer http.ResponseWriter, request *http.Request) {
	orderNumber, err := strconv.ParseInt(chi.URLParam(request, "number"), 10, 64)
	// если orderNumber не парсится в int
	if err != nil {
		writer.Header().Set("Content-Type", "text/plain")
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	bonus, ok := m.mockData[int(orderNumber)]
	if !ok {
		writer.Header().Set("Content-Type", "text/plain")
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	jsonBonus, err := json.Marshal(bonus)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Write(jsonBonus)
}
