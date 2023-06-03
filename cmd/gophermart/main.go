package main

import (
	"github.com/firesworder/loyalty_program/internal/env"
	"github.com/firesworder/loyalty_program/internal/server"
	"github.com/firesworder/loyalty_program/internal/storage"
	"github.com/firesworder/loyalty_program/internal/updatingWorker"
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
	worker := updatingWorker.NewWorker(sqlStorage, 1*time.Minute, environment.AccrualSystemAddress)
	go worker.Start()
	s := server.NewServer(environment.ServerAddress, sqlStorage)
	s.Start()
}
