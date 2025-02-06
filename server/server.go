package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/zok"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

type Server struct {
	srv  *http.Server
	ctx  context.Context
	stop context.CancelFunc

	apply chan struct{}
}

func New() *Server {
	return &Server{
		apply: make(chan struct{}, 1),
		srv: &http.Server{
			Addr: net.JoinHostPort("", config.Config.ServerPort),
		},
	}
}

func (s *Server) init() (err error) {
	go s.ddns(s.ctx)
	s.buildRouter()
	return nil
}

func (s *Server) Run() {
	s.ctx, s.stop = context.WithCancel(context.Background())
	defer s.stop()

	if err := s.init(); err != nil {
		log.Error(err)
		s.stop()
	}

	go func() {
		select {
		case sig := <-zok.Exit():
			log.Infof("shutdown (signal: %s)", sig.String())
			s.stop()
		case <-s.ctx.Done():
		}
	}()

	go s.listen()

	<-s.ctx.Done()

	_ = s.srv.Shutdown(s.ctx)
}

func (s *Server) listen() {
	for {
		err := s.srv.ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}

		var err2 *os.SyscallError
		if errors.As(err, &err2) { // like: errors.Is(err, syscall.EADDRINUSE)
			log.Error(err)
			s.stop()
			return
		}

		log.Warnf("ListenAndServe(): %s", err.Error())
		time.Sleep(time.Second)
	}
}
