package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/firesworder/loyalty_program/internal/storage"
	"io"
	"net/http"
	"strconv"
)

var (
	ErrEmptyOrderNumber             = errors.New("order number can not be empty")
	ErrOrderNumberContainsNotDigits = errors.New("order number contains not only digits")
	ErrOrderIncorrectNumberByLuhn   = errors.New("order number is not correct")
)

func checkOrderNumberByLuhn(orderNumber string) (err error) {
	lPart, rPart := orderNumber[:len(orderNumber)-1], orderNumber[len(orderNumber)-1]

	checkNumber, err := strconv.ParseInt(string(rPart), 10, 64)
	if err != nil {
		return fmt.Errorf("order number can only contains digits")
	}

	lastLPIndex := len(lPart) - 1
	var lSum int64
	for i := 0; i < len(lPart); i++ {
		s := string(lPart[lastLPIndex-i])
		digit, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("order number can only contains digits")
		}

		res := digit
		if i%2 == 0 {
			res *= 2
		}
		if res > 9 {
			res -= 9
		}
		lSum += res
	}

	if (lSum+checkNumber)%10 != 0 {
		return fmt.Errorf("order number incorrect")
	}
	return nil
}

func (s *Server) getUserAuthData(writer http.ResponseWriter, request *http.Request) *storage.User {
	token := request.Context().Value("token")
	if token == nil {
		http.Error(writer, "user not authorized", http.StatusUnauthorized)
		return nil
	} else if tokenStr, ok := token.(string); !ok || !s.TokensCache.IsTokenExist(tokenStr) {
		http.Error(writer, "user not authorized", http.StatusUnauthorized)
		return nil
	}
	user := s.TokensCache.Users[token.(string)]
	return &user
}

// handlerRegisterOrderNumber загрузка пользователем номера заказа для расчёта
func (s *Server) handlerRegisterOrderNumber(writer http.ResponseWriter, request *http.Request) {
	user := s.getUserAuthData(writer, request)
	if user == nil {
		return
	}

	// взять номер заказа из запроса
	defer request.Body.Close()
	reqBody, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(reqBody) == 0 {
		http.Error(writer, "body can not be empty", http.StatusBadRequest)
		return
	}

	// проверить номер заказа на корректность
	orderNumber := string(reqBody)
	err = checkOrderNumberByLuhn(orderNumber)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	err = s.Storage.AddOrder(request.Context(), orderNumber, *user)
	if err != nil {
		if errors.Is(err, storage.ErrOrderRegByThatUser) {
			writer.WriteHeader(http.StatusOK)
			return
		} else if errors.Is(err, storage.ErrOrderRegByOtherUser) {
			http.Error(writer, err.Error(), http.StatusConflict)
			return
		}
	}
	writer.WriteHeader(http.StatusAccepted)
}

// handlerGetOrderStatusList получение списка загруженных пользователем номеров заказов,
// статусов их обработки и информации о начислениях
func (s *Server) handlerGetOrderStatusList(writer http.ResponseWriter, request *http.Request) {
	user := s.getUserAuthData(writer, request)
	if user == nil {
		return
	}

	statusList := s.Storage.GetOrderStatusList(request.Context(), *user)
	rJSON, err := json.Marshal(statusList)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}

	writer.Header().Set("Content-Type", ContentTypeJSON)
	writer.Write(rJSON)
}

// handlerGetBalance получение текущего баланса счёта баллов лояльности пользователя
func (s *Server) handlerGetBalance(writer http.ResponseWriter, request *http.Request) {
	user := s.getUserAuthData(writer, request)
	if user == nil {
		return
	}

	balance, err := s.Storage.GetBalance(request.Context(), *user)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	r := struct {
		Current   int64 `json:"current"`
		Withdrawn int64 `json:"withdrawn"`
	}{Current: balance.BalanceAmount, Withdrawn: balance.WithdrawnAmount}

	rJSON, err := json.Marshal(r)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", ContentTypeJSON)
	writer.Write(rJSON)
}

// handlerWithdrawBonuses запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа
func (s *Server) handlerWithdrawBonuses(writer http.ResponseWriter, request *http.Request) {
	user := s.getUserAuthData(writer, request)
	if user == nil {
		return
	}

	// считать тело запроса
	r := struct {
		Order string `json:"order"`
		Sum   int64  `json:"sum"`
	}{}
	err := json.NewDecoder(request.Body).Decode(&r)
	if err != nil || (r.Order == "" || r.Sum == 0) {
		http.Error(writer, "order and sum fields should be set and not empty", http.StatusBadRequest)
		return
	}

	err = checkOrderNumberByLuhn(r.Order)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	err = s.Storage.AddWithdrawn(request.Context(), r.Order, r.Sum, *user)
	if errors.Is(err, storage.ErrBalanceExceeded) {
		http.Error(writer, err.Error(), http.StatusPaymentRequired)
		return
	}
}

// handlerGetWithdrawals получение информации о выводе средств с накопительного счёта пользователем
func (s *Server) handlerGetWithdrawals(writer http.ResponseWriter, request *http.Request) {
	user := s.getUserAuthData(writer, request)
	if user == nil {
		return
	}

	withdrawalsList := s.Storage.GetWithdrawnList(request.Context(), *user)
	if len(withdrawalsList) == 0 {
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	rJSON, err := json.Marshal(withdrawalsList)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", ContentTypeJSON)
	writer.Write(rJSON)
}
