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
	"sync"
	"syscall"
	"time"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/server"
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

	h := sha1.New()
	if err := f.AddWatch(config.ConfigPath, Remove|Rename|Create|CloseWrite); err != nil {
		log.Error(err)
		return
	}
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
		write(h, config.Config().TLSCertificate)
		write(h, config.Config().TLSKey)
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

	var running bool
	srv := make(chan struct{}, 1)
	srv <- struct{}{}

	var ctx, cancel = context.WithCancelCause(appCtx)
	var wg = &sync.WaitGroup{}

	for {
		select {
		case sig := <-terminate:
			appExit(fmt.Errorf("%w (%s)", ErrTerminated, sig))
			if running {
				wg.Wait()
			}
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
			if bytes.Equal(hash, b) {
				continue
			}

			hash = b

			cancel(ErrConfigChanged)
			ctx, cancel = context.WithCancelCause(appCtx)

			srv <- struct{}{}
		case <-srv:
			wg.Add(1)
			running = true
			go func() {
				defer wg.Done()
				server.New().Run(ctx)
				if exit := errors.Is(context.Cause(ctx), ErrTerminated); exit {
					log.Error(context.Cause(ctx))
				} else if err := context.Cause(ctx); err != nil {
					log.Info("server restart because:", err.Error())
				}
			}()
		}
	}

}
