package main

import (
	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/server"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

func main() {
	if err := config.Parse(); err != nil {
		panic(err)
	}
	log.Open(log.Options{Mode: "file"})
	defer func() {
		if err := log.Close(); err != nil {
			panic(err)
		}
	}()
	server.New().Run()
}
