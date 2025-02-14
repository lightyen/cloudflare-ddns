package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
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

func write(h hash.Hash, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	io.Copy(h, f)
}

func main() {
	config.Load()

	log.Open(log.Options{})
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

	var watched []string

	h := sha1.New()
	if err := f.AddWatch(config.ConfigPath, Remove|Rename|Create|CloseWrite); err != nil {
		log.Error(err)
		return
	}
	watched = append(watched, config.ConfigPath)
	write(h, config.ConfigPath)

	if config.Config().TLSCertificate != "" || config.Config().TLSKey != "" {
		if err := f.AddWatch(config.Config().TLSCertificate, Remove|Rename|Create|CloseWrite); err != nil {
			log.Error(err)
			return
		}
		if err := f.AddWatch(config.Config().TLSKey, Remove|Rename|Create|CloseWrite); err != nil {
			log.Error(err)
			return
		}
		watched = append(watched, config.Config().TLSCertificate)
		watched = append(watched, config.Config().TLSKey)
		write(h, config.Config().TLSCertificate)
		write(h, config.Config().TLSKey)
	}

	hash := h.Sum(nil)

	go f.Watch(ch)

	go func() {
		const duration = 100 * time.Millisecond
		var ctx context.Context
		var cancel context.CancelFunc

		for e := range ch {
			log.Debugf("inotify: %+v", e)

			t := filepath.Clean(filepath.Join(e.Path, e.Name))

			if !slices.ContainsFunc(watched, func(s string) bool {
				return filepath.Clean(s) == t
			}) {
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
				if err := config.Load(); err != nil && errors.Is(err, fs.ErrNotExist) {
					log.Error(err)
				}

				h := sha1.New()
				write(h, config.ConfigPath)
				if config.Config().TLSCertificate != "" || config.Config().TLSKey != "" {
					write(h, config.Config().TLSCertificate)
					write(h, config.Config().TLSKey)
				}

				b := h.Sum(nil)
				if !bytes.Equal(hash, b) {
					hash = b
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
