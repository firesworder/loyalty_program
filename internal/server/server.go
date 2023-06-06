package server

import (
	"context"
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"log"
	"net/http"
	"net/url"
)

type tokenKey string

type Server struct {
	Address     string
	Storage     storage.Storage
	Router      chi.Router
	TokensCache *AuthTokensCache
	tokenGenKey []byte
}

func NewServer(addr string, storage storage.Storage) (s *Server, err error) {
	s = &Server{Address: addr}
	s.Storage = storage
	s.TokensCache = NewAuthTokensCache(nil)
	if s.tokenGenKey, err = generateRandom(32); err != nil {
		return nil, err
	}
	s.InitRouter()
	return s, nil
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

func (s *Server) InitAuthToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenCookie, err := request.Cookie("token")
		if err == nil {
			token, err := url.QueryUnescape(tokenCookie.Value)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
				return
			}
			ctx := request.Context()
			key := tokenKey("token")
			ctx = context.WithValue(ctx, key, token)
			request = request.WithContext(ctx)
		} else if err != http.ErrNoCookie {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		// если нет куки - ничего не делать
		next.ServeHTTP(writer, request)
	})
}
