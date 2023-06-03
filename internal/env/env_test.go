package env

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func TestParseEnvArgs(t *testing.T) {
	env := NewEnvironment()

	tests := []struct {
		name    string
		cmdStr  string
		envVars map[string]string
		wantEnv *Environment
	}{
		{
			name:    "Test correct 1. Empty cmd args and env vars.",
			cmdStr:  "file.exe",
			envVars: map[string]string{},
			wantEnv: &Environment{
				ServerAddress: "localhost:8080",
			},
		},
		{
			name:    "Test correct 2. Set cmd args and empty env vars.",
			cmdStr:  "file.exe -a=cmd.site",
			envVars: map[string]string{},
			wantEnv: &Environment{
				ServerAddress: "cmd.site",
			},
		},
		{
			name:    "Test correct 3. Empty cmd args and set env vars.",
			cmdStr:  "file.exe",
			envVars: map[string]string{"ADDRESS": "env.site"},
			wantEnv: &Environment{
				ServerAddress: "env.site",
			},
		},
		{
			name:    "Test correct 4. Set cmd args and set env vars.",
			cmdStr:  "file.exe -a=cmd.site",
			envVars: map[string]string{"ADDRESS": "env.site"},
			wantEnv: &Environment{
				ServerAddress: "env.site",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// удаляю переменные окружения, если они были до этого установлены
			for _, key := range [1]string{"ADDRESS"} {
				err := os.Unsetenv(key)
				require.NoError(t, err)
			}
			// устанавливаю переменные окружения использованные для теста
			for key, value := range tt.envVars {
				err := os.Setenv(key, value)
				require.NoError(t, err)
			}
			// устанавливаю os.Args как эмулятор вызванной команды
			os.Args = strings.Split(tt.cmdStr, " ")
			env.ParseEnvArgs()
			assert.Equal(t, tt.wantEnv, env)
		})
	}
}
