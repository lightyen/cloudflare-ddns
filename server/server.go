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
	srv *http.Server

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

func (s *Server) init(ctx context.Context) (err error) {
	// go s.ddns(ctx)
	s.buildRouter()
	return nil
}

func (s *Server) Run(ctx context.Context) error {
	log.Info("server startup...")

	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)

	if err := s.init(ctx); err != nil {
		stop(errors.New("init failed"))
		return err
	}

	go func() {
		<-ctx.Done()
		_ = s.srv.Shutdown(ctx)
	}()

	for {
		select {
		default:
		case <-ctx.Done():
			return ctx.Err()
		}

		ln, err := net.Listen("tcp", s.srv.Addr)
		if err == nil {
			log.Info("server listen:", s.srv.Addr)
			err = s.srv.Serve(ln)
		}

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
