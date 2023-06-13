package main

import (
	"github.com/firesworder/loyalty_program/internal/env"
	"github.com/firesworder/loyalty_program/internal/server"
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/firesworder/loyalty_program/internal/updatingworker"
	"log"
	"time"
)

func main() {
	environment := env.NewEnvironment()
	environment.ParseEnvArgs()
	if environment.ServerAddress == "" || environment.DSN == "" || environment.AccrualSystemAddress == "" {
		log.Fatal("not all env var were set")
	}
	sqlStorage, err := storage.NewSQLStorage(environment.DSN)
	if err != nil {
		log.Fatal(err)
	}
	worker := updatingworker.NewWorker(sqlStorage, 5*time.Second, environment.AccrualSystemAddress)
	go worker.Start()
	s, err := server.NewServer(environment.ServerAddress, sqlStorage)
	if err != nil {
		log.Fatal(err)
	}
	s.Start()
}
