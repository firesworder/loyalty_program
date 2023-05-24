package server

import (
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/stretchr/testify/assert"
	"testing"
)

var userAdmin = &storage.MockUser{Login: "admin", HashedPassword: "admin"}
var userPostgres = &storage.MockUser{Login: "postgres", HashedPassword: "postgres"}

func TestAuthTokensCache_AddUser(t *testing.T) {
	type args struct {
		authToken string
		user      storage.User
	}
	tests := []struct {
		name string
		args
		initCacheState   map[string]storage.User
		wantedCacheState map[string]storage.User
		wantError        bool
	}{
		{
			name:             "Test 1. Token is not present in cache.",
			args:             args{authToken: "adminToken", user: userAdmin},
			initCacheState:   map[string]storage.User{"postgresToken": userPostgres},
			wantedCacheState: map[string]storage.User{"adminToken": userAdmin, "postgresToken": userPostgres},
			wantError:        false,
		},
		{
			name:             "Test 2. Token is present in cache.",
			args:             args{authToken: "postgresToken", user: userPostgres},
			initCacheState:   map[string]storage.User{"postgresToken": userPostgres},
			wantedCacheState: map[string]storage.User{"postgresToken": userPostgres},
			wantError:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := AuthTokensCache{Users: tt.initCacheState}
			err := c.AddUser(tt.args.authToken, tt.args.user)
			assert.Equal(t, tt.wantError, err != nil)
		})
	}
}

func TestAuthTokensCache_IsTokenExist(t *testing.T) {
	type args struct {
		authToken string
	}
	tests := []struct {
		name string
		args
		initCacheState map[string]storage.User
		want           bool
	}{
		{
			name:           "Test 1. Token is not present in cache.",
			args:           args{authToken: "adminToken"},
			initCacheState: map[string]storage.User{"postgresToken": userPostgres},
			want:           false,
		},
		{
			name:           "Test 2. Token is present in cache.",
			args:           args{authToken: "postgresToken"},
			initCacheState: map[string]storage.User{"postgresToken": userPostgres},
			want:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := AuthTokensCache{Users: tt.initCacheState}
			got := c.IsTokenExist(tt.args.authToken)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewAuthTokensCache(t *testing.T) {
	type args struct {
		users map[string]storage.User
	}
	tests := []struct {
		name       string
		cacheState map[string]storage.User
		wantCache  *AuthTokensCache
	}{
		{
			name: "Simple constructor test.",
			cacheState: map[string]storage.User{
				"adminToken":    userAdmin,
				"postgresToken": userPostgres,
			},
			wantCache: &AuthTokensCache{
				Users: map[string]storage.User{
					"adminToken":    userAdmin,
					"postgresToken": userPostgres,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewAuthTokensCache(tt.cacheState)
			assert.Equal(t, tt.wantCache, c)
		})
	}
}
