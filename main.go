package main

import (
	"os"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/server"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

func main() {
	if err := config.Parse(); err != nil {
		panic(err)
	}
	log.Open(log.Options{Mode: "stdout"})
	defer func() {
		if err := log.Close(); err != nil {
			panic(err)
		}
	}()
	wd, _ := os.Getwd()
	log.Info("Working Directory:", wd)
	log.Info("Zone ID:", config.Config.ZoneID)
	log.Info("Email:", config.Config.Email)
	log.Info("Token:", config.Config.Token)
	server.New().Run()
}
