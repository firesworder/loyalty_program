package server

import (
	"crypto/md5"
	"encoding/hex"
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
const TokenExpires = 7 * 24 * time.Hour // неделя
const HashSalt = "b509c8147abf2cf02d9f12707afdf4ae"

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

func createToken(u *postArgsUser) string {
	token := u.Login + u.Password + time.Now().String() + HashSalt
	hashToken := md5.Sum([]byte(token))
	return hex.EncodeToString(hashToken[:])
}

func setTokenCookie(writer http.ResponseWriter, token string) {
	expires := time.Now().Add(TokenExpires)
	cookie := http.Cookie{Name: TokenCookieName, Value: url.QueryEscape(token), Expires: expires}
	http.SetCookie(writer, &cookie)
}

// handlerRegisterUser хандлер регистрации пользователей
func (s *Server) handlerRegisterUser(writer http.ResponseWriter, request *http.Request) {
	userPost := checkReqAuthData(writer, request)
	if userPost == nil {
		return
	}

	// хеш пароля
	hash := md5.Sum([]byte(userPost.Password))
	hashedPassword := hex.EncodeToString(hash[:])

	user, err := s.Storage.AddUser(userPost.Login, hashedPassword)
	if err != nil {
		if errors.Is(err, storage.ErrLoginExist) {
			http.Error(writer, err.Error(), http.StatusConflict)
			return
		} else {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	token := createToken(userPost)

	s.TokensCache.AddUser(token, *user)
	setTokenCookie(writer, token)
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

	token := createToken(userPost)

	s.TokensCache.AddUser(token, *user)
	setTokenCookie(writer, token)
}
