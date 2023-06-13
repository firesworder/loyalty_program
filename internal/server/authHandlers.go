package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/firesworder/loyalty_program/internal/storage"
	"golang.org/x/crypto/bcrypt"
	"log"
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
const bcryptCost = 8

// generateRandom - создает массив байт заданной длины
func generateRandom(size int) ([]byte, error) {
	randBytes := make([]byte, size)
	_, err := rand.Read(randBytes)
	if err != nil {
		return nil, err
	}
	return randBytes, nil
}

// generateToken - создает токен авторизации, с использованием hmac
func generateToken(key []byte) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("token gen key is not set")
	}

	newToken, err := generateRandom(32)
	if err != nil {
		return nil, err
	}

	h := hmac.New(sha256.New, key)
	h.Write(newToken)
	return h.Sum(nil), err
}

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

func setTokenCookie(writer http.ResponseWriter, token string) {
	expires := time.Now().Add(TokenExpires)
	cookie := http.Cookie{Name: TokenCookieName, Value: url.QueryEscape(token), Expires: expires}
	http.SetCookie(writer, &cookie)
}

// handlerRegisterUser хандлер регистрации пользователей
func (s *Server) handlerRegisterUser(writer http.ResponseWriter, request *http.Request) {
	// todo: перенести в констрейнты бд?
	userPost := checkReqAuthData(writer, request)
	if userPost == nil {
		return
	}

	// хеш пароля
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userPost.Password), bcryptCost)
	if err != nil {
		log.Println(err)
		http.Error(writer, "", http.StatusInternalServerError)
		return
	}

	user, err := s.Storage.AddUser(request.Context(), userPost.Login, string(hashedPassword))
	if err != nil {
		if errors.Is(err, storage.ErrLoginExist) {
			http.Error(writer, err.Error(), http.StatusConflict)
			return
		} else {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	token, err := generateToken(s.tokenGenKey)
	if err != nil {
		log.Println(err)
		http.Error(writer, "", http.StatusInternalServerError)
		return
	}
	tokenStr := hex.EncodeToString(token)

	s.TokensCache.AddUser(tokenStr, *user)
	setTokenCookie(writer, tokenStr)
}

// handlerLoginUser хандлер авторизации
func (s *Server) handlerLoginUser(writer http.ResponseWriter, request *http.Request) {
	userPost := checkReqAuthData(writer, request)
	if userPost == nil {
		return
	}

	user, err := s.Storage.GetUser(request.Context(), userPost.Login)
	// todo: исправить вложенность ошибки
	if errors.Is(err, storage.ErrLoginNotExist) {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(userPost.Password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			http.Error(writer, "password incorrect", http.StatusUnauthorized)
			return
		}
		log.Println(err)
		http.Error(writer, "", http.StatusInternalServerError)
		return
	}

	token, err := generateToken(s.tokenGenKey)
	if err != nil {
		log.Println(err)
		http.Error(writer, "", http.StatusInternalServerError)
		return
	}
	tokenStr := hex.EncodeToString(token)

	s.TokensCache.AddUser(tokenStr, *user)
	setTokenCookie(writer, tokenStr)
}
