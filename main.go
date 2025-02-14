package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/server"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

var (
	ErrExit          = errors.New("exit")
	ErrConfigChanged = errors.New("config changed")

	mu              = &sync.RWMutex{}
	appCtx, appExit = context.WithCancelCause(context.Background())
)

func AppCtx() context.Context {
	mu.RLock()
	defer mu.RUnlock()
	return appCtx
}

func AppExit() context.CancelCauseFunc {
	mu.RLock()
	defer mu.RUnlock()
	return appExit
}

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

	if err := f.AddWatch(filepath.Dir(config.ConfigPath), Remove|Rename|Create|CloseWrite); err != nil {
		log.Error(err)
		return
	}

	go f.Watch(ch)

	go func() {
		const duration = 100 * time.Millisecond
		var ctx context.Context
		var cancel context.CancelFunc

		for e := range ch {
			log.Debugf("inotify: %+v", e)
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
			ctx, cancel = context.WithTimeout(AppCtx(), duration)
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
			case sig := <-Exit():
				AppExit()(fmt.Errorf("%w (signal: %s)", ErrExit, sig))
				return
			case <-changed:
				if !config.Equal() {
					AppExit()(ErrConfigChanged)
				}
			}
		}
	}()

	for run(AppCtx()) {
		mu.Lock()
		appCtx, appExit = context.WithCancelCause(context.Background())
		mu.Unlock()
		time.Sleep(time.Second)
	}
}

func run(ctx context.Context) bool {
	if err := config.Load(); err != nil {
		log.Error(err)
	}

	srv := server.New()
	srv.Run(ctx)

	restart := !errors.Is(context.Cause(ctx), ErrExit)

	if restart {
		if err := context.Cause(ctx); err != nil {
			log.Info("server restart because:", err.Error())
		}
	} else {
		log.Error(context.Cause(ctx))
	}

	return restart
}

func Exit() <-chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop,
		syscall.SIGTERM, // kill
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGQUIT, // Ctrl+\
	)
	return stop
}
