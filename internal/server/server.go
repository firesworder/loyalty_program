package server

import (
	"context"
	"errors"
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"log"
	"net/http"
	"net/url"
)

// todo: настроить логирование

// todo: перенести в блок авторизации
type tokenKey string

type Server struct {
	Address     string
	Storage     storage.Storage
	Router      chi.Router
	TokensCache *AuthTokensCache
}

func NewServer(addr string, storage storage.Storage) *Server {
	s := &Server{Address: addr}
	s.Storage = storage
	s.TokensCache = NewAuthTokensCache(nil)
	s.InitRouter()
	return s
}

func (s *Server) InitRouter() {
	s.Router = chi.NewRouter()

	s.Router.Use(s.InitAuthToken)
	s.Router.Use(middleware.RequestID)
	s.Router.Use(middleware.RealIP)
	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.Recoverer)

	s.Router.Route("/api/user/", func(r chi.Router) {
		r.Post("/register", s.handlerRegisterUser)
		r.Post("/login", s.handlerLoginUser)

		r.Post("/orders", s.handlerRegisterOrderNumber)
		r.Get("/orders", s.handlerGetOrderStatusList)
		r.Get("/balance", s.handlerGetBalance)
		r.Post("/balance/withdraw", s.handlerWithdrawBonuses)
		r.Get("/withdrawals", s.handlerGetWithdrawals)
	})
}

func (s *Server) Start() {
	server := http.Server{Addr: s.Address, Handler: s.Router}
	log.Fatal(server.ListenAndServe())
}

func handleInternalError(err error, w http.ResponseWriter) {
	log.Println(err)
	http.Error(w, "", http.StatusInternalServerError)
}

// todo: написать тесты для функции
func (s *Server) InitAuthToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenCookie, err := request.Cookie("token")
		// если нет куки(ошибка ErrNoCookie) - ничего не делать
		if err != nil && !errors.Is(err, http.ErrNoCookie) {
			handleInternalError(err, writer)
			return
		} else if err == nil {
			token, err := url.QueryUnescape(tokenCookie.Value)
			if err != nil {
				handleInternalError(err, writer)
				return
			}

			// меняю контекст запроса
			ctx := request.Context()
			key := tokenKey("token")
			ctx = context.WithValue(ctx, key, token)
			request = request.WithContext(ctx)
		}
		next.ServeHTTP(writer, request)
	})
}
