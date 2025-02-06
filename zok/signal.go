package zok

import (
	"os"
	"os/signal"
	"syscall"
)

func Exit() <-chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop,
		syscall.SIGTERM, // kill
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGQUIT, // Ctrl+\
	)
	return stop
}
