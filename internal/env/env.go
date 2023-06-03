package env

import (
	"flag"
	"github.com/caarlos0/env/v8"
	"log"
)

type Environment struct {
	ServerAddress        string `env:"RUN_ADDRESS"`
	DSN                  string `env:"DATABASE_URI"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

func NewEnvironment() *Environment {
	e := &Environment{}
	e.InitCmdArgs()
	return e
}

func (e *Environment) InitCmdArgs() {
	flag.StringVar(&e.ServerAddress, "a", "", "Server address")
	flag.StringVar(&e.ServerAddress, "d", "", "Server address")
	flag.StringVar(&e.ServerAddress, "r", "", "Server address")
}

// ParseEnvArgs Парсит значения полей Env. Сначала из cmd аргументов, затем из перем-х окружения
func (e *Environment) ParseEnvArgs() {
	// Парсинг аргументов cmd
	flag.Parse()

	// Парсинг перем окружения
	err := env.Parse(e)
	if err != nil {
		log.Fatal(err)
	}
}
