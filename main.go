package main

import (
	"errors"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/zok"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

type Op uint

const (
	Create Op = 1 << iota
	Modify
	Remove
	Rename
	Chmod
)

func (o Op) String() string {
	switch o {
	case Create:
		return "Create"
	case Modify:
		return "Modify"
	case Remove:
		return "Remove"
	case Rename:
		return "Rename"
	case Chmod:
		return "Chmod"
	default:
		return "Unknown"
	}
}

type INotify struct {
	fd   int
	file *os.File

	watchdesc map[int]string
	wd        map[string]int
}

func NewINotify() *INotify {
	return &INotify{
		watchdesc: map[int]string{},
		wd:        map[string]int{},
	}
}

func (f *INotify) Open() (err error) {
	f.fd, err = syscall.InotifyInit1(0)
	if err != nil {
		return err
	}
	f.file = os.NewFile(uintptr(f.fd), "")
	go f.readEvents()
	return nil
}

func (f *INotify) Close() error {
	for w := range f.watchdesc {
		syscall.InotifyRmWatch(f.fd, uint32(w))
	}
	f.watchdesc = map[int]string{}
	f.wd = map[string]int{}
	return syscall.Close(f.fd)
}

func (f *INotify) AddWatch(path string) error {
	_, exists := f.wd[path]
	if exists {
		return nil
	}
	w, err := syscall.InotifyAddWatch(f.fd, path, syscall.IN_MODIFY|syscall.IN_CREATE)
	if err != nil {
		return err
	}
	f.wd[path] = w
	f.watchdesc[w] = path
	return nil
}

func (f *INotify) readEvents() {
	buf := make([]byte, syscall.SizeofInotifyEvent<<12)
	for {
		n, err := f.file.Read(buf)
		switch {
		case errors.Is(err, os.ErrClosed):
			return
		case err != nil:
			log.Error(err)
			continue
		}

		if n < syscall.SizeofInotifyEvent {
			continue
		}

		var offset int

		for offset <= (n - syscall.SizeofInotifyEvent) {
			var name string
			p := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))

			if p.Mask&syscall.IN_IGNORED == syscall.IN_IGNORED {
				offset += int(syscall.SizeofInotifyEvent + p.Len)
				continue
			}

			if p.Len > 0 {
				b := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[offset+syscall.SizeofInotifyEvent]))
				name = string(b[0:p.Len])
			}

			e := newEvent(name, p)

			log.Info(time.Now(), e, f.watchdesc[int(p.Wd)], p)

			offset += int(syscall.SizeofInotifyEvent + p.Len)
		}

	}
}

type InotifyEvent struct {
	Name string
	Len  uint32
	Op   Op
}

func newEvent(name string, event *syscall.InotifyEvent) (e InotifyEvent) {
	e.Name = name
	e.Len = event.Len

	flag := func(v uint32) bool {
		return event.Mask&v == v
	}

	if flag(syscall.IN_CREATE) || flag(syscall.IN_MOVED_TO) {
		e.Op |= Create
	}
	if flag(syscall.IN_DELETE_SELF) || flag(syscall.IN_DELETE) {
		e.Op |= Remove
	}
	if flag(syscall.IN_MODIFY) {
		e.Op |= Modify
	}
	if flag(syscall.IN_MOVE_SELF) || flag(syscall.IN_MOVED_FROM) {
		e.Op |= Rename
	}
	if flag(syscall.IN_ATTRIB) {
		e.Op |= Chmod
	}
	return
}

func main() {
	if err := config.Parse(); err != nil {
		panic(err)
	}
	log.Open(log.Options{Mode: "stdout"})
	defer func() {
		if err := log.Close(); err != nil {
			panic(err)
		}
	}()
	log.Info("Zone ID:", config.Config.ZoneID)
	log.Info("Email:", config.Config.Email)
	log.Info("Token:", config.Config.Token)
	// server.New().Run()

	f := NewINotify()

	if err := f.Open(); err != nil {
		log.Error(err)
		return
	}
	defer f.Close()

	if err := f.AddWatch("config.json"); err != nil {
		log.Error(err)
		return
	}

	<-zok.Exit()
}
