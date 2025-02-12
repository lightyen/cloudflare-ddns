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

	if err := f.AddWatch(filepath.Dir(config.ConfigPath), Remove|Rename|Create|CloseWrite); err != nil {
		log.Error(err)
		return
		// if !errors.Is(err, fs.ErrNotExist) {
		// 	log.Error(err)
		// 	return
		// }

		// _ = config.NewFile()
		// if err := f.AddWatch(filepath.Dir(config.ConfigPath), Remove|Rename|Create|CloseWrite); err != nil {
		// 	log.Error(err)
		// 	return
		// }
	}

	go func() {
		duration := time.Second
		var ctx context.Context
		var cancel context.CancelFunc

		for e := range ch {
			name := e.Name
			if name == "" {
				name = filepath.Base(e.Path)
			}

			if name != filepath.Base(config.ConfigPath) {
				continue
			}

			if cancel != nil {
				cancel()
			}

			// debounce
			ctx, cancel = context.WithTimeout(context.Background(), duration)
			defer cancel()

			go func(ctx context.Context) {
				time.Sleep(duration)
				select {
				case <-ctx.Done():
					return
				default:
				}
				changed <- struct{}{}
			}(ctx)
		}
	}()

	go func() {
		for {
			select {
			case sig := <-zok.Exit():
				mu.Lock()
				cancel(fmt.Errorf("%w (signal: %s)", ErrExit, sig))
				ctx, cancel = context.WithCancelCause(context.Background())
				mu.Unlock()
				return
			case <-changed:
				if !config.Equal() {
					mu.Lock()
					cancel(ErrConfigChanged)
					ctx, cancel = context.WithCancelCause(context.Background())
					mu.Unlock()
				}
			}
		}
	}()

	for run() {
		time.Sleep(time.Second)
	}
}

func run() bool {
	var c context.Context
	mu.RLock()
	c = ctx
	mu.RUnlock()

	if err := config.Load(); err != nil {
		log.Error(err)
	}

	srv := server.New()
	srv.Run(c)

	if errors.Is(context.Cause(c), ErrExit) {
		return false
	}

	return true
}
