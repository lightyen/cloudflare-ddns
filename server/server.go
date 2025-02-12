package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

type Server struct {
	srv  *http.Server
	ctx  context.Context
	stop context.CancelCauseFunc

	apply chan struct{}
}

func New() *Server {
	return &Server{
		apply: make(chan struct{}, 1),
		srv: &http.Server{
			Addr: net.JoinHostPort("", strconv.FormatInt(int64(config.Config().ServerPort), 10)),
		},
	}
}

func (s *Server) init() (err error) {
	// go s.ddns(s.ctx)
	s.buildRouter()
	return nil
}

func (s *Server) Run(ctx context.Context) error {
	log.Info("server startup...")

	s.ctx, s.stop = context.WithCancelCause(ctx)
	defer s.stop(nil)

	if err := s.init(); err != nil {
		s.stop(errors.New("init failed"))
		return err
	}

	go func() {
		<-s.ctx.Done()
		log.Info("server stop because:", context.Cause(s.ctx).Error())
		_ = s.srv.Shutdown(s.ctx)
	}()

	for {
		select {
		default:
		case <-s.ctx.Done():
			return s.ctx.Err()
		}

		log.Info("server listen:", s.srv.Addr)
		err := s.srv.ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return err
		}

		// var err2 *os.SyscallError
		// if errors.As(err, &err2) { // like: errors.Is(err, syscall.EADDRINUSE)
		// 	s.stop(err)
		// 	return err
		// }

		log.Warn("server listen:", err)
		time.Sleep(time.Second)
	}
}
