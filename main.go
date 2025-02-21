package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/lightyen/cloudflare-ddns/server"
	"github.com/lightyen/cloudflare-ddns/settings"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

var (
	ErrTerminated    = errors.New("terminate by signal")
	ErrConfigChanged = errors.New("config changed")

	terminate = func() <-chan os.Signal {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop,
			syscall.SIGTERM, // kill
			syscall.SIGINT,  // Ctrl+C
			syscall.SIGQUIT, // Ctrl+\
		)
		return stop
	}()

	appCtx, appExit = context.WithCancelCause(context.Background())
)

func write(h hash.Hash, data []byte) {
	io.Copy(h, bytes.NewReader(data))
}

func writeFile(h hash.Hash, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	io.Copy(h, f)
}

func main() {
	settings.Load()
	if err := settings.FlagParse(); err != nil {
		if errors.Is(err, settings.ErrShowVersion) || errors.Is(err, settings.ErrHelp) {
			return
		}
		os.Exit(1)
	}

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

	h := sha1.New()
	if err := f.AddWatch(settings.ConfigPath(), Remove|Rename|Create|CloseWrite); err != nil {
		log.Error(err)
		return
	}

	if settings.Value().TLSCertificate != "" || settings.Value().TLSKey != "" {
		if err := f.AddWatch(settings.Value().TLSCertificate, Remove|Rename|Create|CloseWrite); err != nil {
			log.Error(err)
			return
		}
		if err := f.AddWatch(settings.Value().TLSKey, Remove|Rename|Create|CloseWrite); err != nil {
			log.Error(err)
			return
		}
	}

	for _, s := range f.Watched() {
		if s == settings.ConfigPath() {
			m, _ := settings.ReadConfigFile()
			data, _ := json.Marshal(m)
			write(h, data)
			continue
		}
		writeFile(h, s)
	}

	hash := h.Sum(nil)

	go f.Watch(ch)

	go func() {
		const duration = 200 * time.Millisecond
		var ctx context.Context
		var cancel context.CancelFunc

		for range ch {
			if cancel != nil {
				cancel()
			}

			// debounce
			ctx, cancel = context.WithTimeout(appCtx, duration)
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

	var ctx, cancel = context.WithCancelCause(appCtx)
	srv := make(chan context.Context, 1)
	srv <- ctx

	var wg = &sync.WaitGroup{}

	for {
		select {
		case sig := <-terminate:
			appExit(fmt.Errorf("%w (%s)", ErrTerminated, sig))
			wg.Wait()
			return
		case ctx := <-srv:
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				server.New().Run(ctx)
				err := context.Cause(ctx)
				if errors.Is(err, ErrTerminated) {
					log.Error(err)
				} else if errors.Is(err, ErrConfigChanged) {
					//
				} else if err != nil {
					log.Info("server restart because:", err.Error())
				}
			}(ctx)
		case <-changed:
			if err := settings.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
				log.Error(err)
			}
			if err := settings.FlagParse(); err != nil {
				log.Error(err)
			}

			h := sha1.New()
			for _, s := range f.Watched() {
				if s == settings.ConfigPath() {
					m, _ := settings.ReadConfigFile()
					data, _ := json.Marshal(m)
					write(h, data)
					continue
				}
				writeFile(h, s)
			}
			b := h.Sum(nil)

			if bytes.Equal(hash, b) {
				continue
			}

			hash = b

			cancel(ErrConfigChanged)
			ctx, cancel = context.WithCancelCause(appCtx)
			srv <- ctx
		}
	}

}
