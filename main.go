package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/server"
	"github.com/lightyen/cloudflare-ddns/zok"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

var (
	ErrExit          = errors.New("exit")
	ErrConfigChanged = errors.New("config changed")

	mu          = &sync.RWMutex{}
	ctx, cancel = context.WithCancelCause(context.Background())
)

func main() {
	log.Open(log.Options{Mode: "stdout"})
	defer func() {
		if err := log.Close(); err != nil {
			panic(err)
		}
	}()

	var ch = make(chan InotifyEvent, 1)
	var changed = make(chan struct{}, 1)

	f := NewINotify()
	if err := f.Open(); err != nil {
		log.Error(err)
		return
	}
	defer f.Close()

	go f.Watch(ch)

	if err := f.AddWatch(filepath.Dir(config.ConfigPath)); err != nil {
		log.Error(err)
		return
	}

	go func() {
		for e := range ch {
			if e.Name == "config.json" {
				changed <- struct{}{}
			}
		}
	}()

	go func() {
		for {
			select {
			case sig := <-zok.Exit():
				mu.RLock()
				cancel(fmt.Errorf("%w (signal: %s)", ErrExit, sig))
				mu.RUnlock()
				return
			case <-changed:
				mu.RLock()
				cancel(ErrConfigChanged)
				mu.RUnlock()
			}
		}
	}()

	for run() {
		time.Sleep(1000 * time.Millisecond)
	}
}

func run() bool {
	if err := config.Load(); err != nil {
		log.Error(err)
	}

	srv := server.New()
	srv.Run(ctx)

	if errors.Is(context.Cause(ctx), ErrExit) {
		return false
	}

	mu.Lock()
	defer mu.Unlock()
	ctx, cancel = context.WithCancelCause(context.Background())

	return true
}
