package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

type Server struct {
	handler http.Handler
	apply   chan struct{}
}

func New() *Server {
	return &Server{
		apply: make(chan struct{}, 1),
	}
}

func (s *Server) init(ctx context.Context) (err error) {
	s.handler = s.buildRouter()
	// go s.ddns(ctx)
	return nil
}

func (s *Server) Run(ctx context.Context) {
	if err := s.init(ctx); err != nil {
		panic(err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = s.serveHTTP(ctx)
	}()
	go func() {
		defer wg.Done()
		err := s.serveHTTPS(ctx)
		if errors.Is(err, fs.ErrNotExist) {
			log.Info("TLS certificate is not found.")
			return
		}
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error(err)
		}
	}()
	wg.Wait()
}

func (s *Server) redirect(handler http.Handler) http.Handler {
	const redirect = false

	h := gin.New()
	h.Any("/*any", func(c *gin.Context) {
		if redirect {
			host, _, err := net.SplitHostPort(c.Request.Host)
			if err != nil {
				host = c.Request.Host
			}
			u := *c.Request.URL
			u.Scheme = "https"
			u.Host = net.JoinHostPort(host, strconv.Itoa(config.Config().ServeTLSPort))
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusMovedPermanently, u.String())
			return
		}
		handler.ServeHTTP(c.Writer, c.Request)
	})

	return h
}

func serve(srv *http.Server, onListenSuccess func()) error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	if onListenSuccess != nil {
		onListenSuccess()
	}
	return srv.Serve(ln)
}

func (s *Server) serveHTTP(ctx context.Context) error {
	srv := &http.Server{
		Addr:    net.JoinHostPort("", strconv.FormatInt(int64(config.Config().ServePort), 10)),
		Handler: s.redirect(s.handler),
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(ctx)
	}()

	for {
		select {
		default:
		case <-ctx.Done():
			return ctx.Err()
		}

		err := serve(srv, func() {
			log.Info("http server listen:", srv.Addr)
		})

		if err == nil {
			panic("unexpected behavior")
		}

		if errors.Is(err, http.ErrServerClosed) {
			return err
		}

		log.Warn("http server listen:", err)
		time.Sleep(time.Second)
	}
}

func (s *Server) serveHTTPS(ctx context.Context) error {
	GetCertificate, err := X509KeyPair(config.Config().TLSCertificate, config.Config().TLSKey)
	if err != nil {
		return fmt.Errorf("serve TLS: %w", err)
	}

	srv := &http.Server{
		Addr:    net.JoinHostPort("", strconv.FormatInt(int64(config.Config().ServeTLSPort), 10)),
		Handler: s.handler,
		TLSConfig: &tls.Config{
			GetCertificate: GetCertificate,
		},
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(ctx)
	}()

	for {
		select {
		default:
		case <-ctx.Done():
			return ctx.Err()
		}

		err := serve(srv, func() {
			log.Info("https server listen:", srv.Addr)
		})

		if err == nil {
			panic("unexpected behavior")
		}

		if errors.Is(err, http.ErrServerClosed) {
			return err
		}

		log.Warn("https server listen:", err)
		time.Sleep(time.Second)
	}
}
