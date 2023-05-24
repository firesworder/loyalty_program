package server

import (
	"encoding/json"
	"errors"
	"github.com/firesworder/loyalty_program/internal/storage"
	"net/http"
	"net/url"
	"time"
)

type postArgsUser struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

const TokenCookieName = "token"

func checkReqAuthData(writer http.ResponseWriter, request *http.Request) *postArgsUser {
	var u postArgsUser
	err := json.NewDecoder(request.Body).Decode(&u)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return nil
	}
	if len(u.Login) == 0 || len(u.Password) == 0 {
		http.Error(writer, "login and passwords fields can not be empty", http.StatusBadRequest)
		return nil
	}
	return &u
}

func setAuthTokenCookie(writer http.ResponseWriter, token string) {
	expires := time.Now().Add(7 * 24 * time.Hour) // кука живет неделю
	cookie := http.Cookie{Name: TokenCookieName, Value: url.QueryEscape(token), Expires: expires}
	http.SetCookie(writer, &cookie)
}

// handlerRegisterUser хандлер регистрации пользователей
func (s *Server) handlerRegisterUser(writer http.ResponseWriter, request *http.Request) {
	userPost := checkReqAuthData(writer, request)
	if userPost == nil {
		return
	}

	user, err := s.Storage.AddUser(userPost.Login, userPost.Password)
	if err != nil {
		if errors.Is(err, storage.ErrLoginExist) {
			http.Error(writer, err.Error(), http.StatusConflict)
			return
		} else {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	err = s.TokensCache.AddUser(user.GetToken(), user)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	setAuthTokenCookie(writer, user.GetToken())
}

// handlerLoginUser хандлер авторизации
func (s *Server) handlerLoginUser(writer http.ResponseWriter, request *http.Request) {
	userPost := checkReqAuthData(writer, request)
	if userPost == nil {
		return
	}

	user, err := s.Storage.GetUser(userPost.Login, userPost.Password)
	if errors.Is(err, storage.ErrAuthDataIncorrect) {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	err = s.TokensCache.AddUser(user.GetToken(), user)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	setAuthTokenCookie(writer, user.GetToken())
}
