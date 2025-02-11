package main

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lightyen/cloudflare-ddns/zok/log"
)

type Op uint

const (
	Create Op = 1 << iota
	Write
	Remove
	Rename
	CloseWrite
	Chmod
)

func (o Op) Has(v Op) bool {
	return o&v == v
}

func (o Op) String() string {
	b := strings.Builder{}

	if o.Has(Create) {
		b.WriteString("|Create")
	}
	if o.Has(Write) {
		b.WriteString("|Write")
	}
	if o.Has(Remove) {
		b.WriteString("|Remove")
	}
	if o.Has(Rename) {
		b.WriteString("|Rename")
	}
	if o.Has(Chmod) {
		b.WriteString("|Chmod")
	}
	if o.Has(CloseWrite) {
		b.WriteString("|CloseWrite")
	}

	return b.String()[1:]
}

type InotifyEvent struct {
	Name string
	Len  uint32
	Op   Op
	Path string
}

func newEvent(name string, path string, event *syscall.InotifyEvent) (e InotifyEvent) {
	e.Name = name
	e.Len = event.Len
	e.Path = path

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
		e.Op |= Write
	}
	if flag(syscall.IN_CLOSE_WRITE) {
		e.Op |= CloseWrite
	}
	if flag(syscall.IN_MOVE_SELF) || flag(syscall.IN_MOVED_FROM) {
		e.Op |= Rename
	}
	if flag(syscall.IN_ATTRIB) {
		e.Op |= Chmod
	}
	return
}

type INotify struct {
	fd   int
	file *os.File

	watchdesc map[int]string
	wd        map[string]int
}

var watch chan InotifyEvent = make(chan InotifyEvent, 1)

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
	return nil
}

func (f *INotify) Close() error {
	for w := range f.watchdesc {
		syscall.InotifyRmWatch(f.fd, uint32(w))
	}
	f.watchdesc = map[int]string{}
	f.wd = map[string]int{}
	return f.file.Close()
}

func (f *INotify) AddWatch(path string) error {
	_, exists := f.wd[path]
	if exists {
		return nil
	}
	w, err := syscall.InotifyAddWatch(f.fd, path, syscall.IN_CLOSE_WRITE|syscall.IN_DELETE)
	if err != nil {
		return err
	}
	f.wd[path] = w
	f.watchdesc[w] = path
	return nil
}

func (f *INotify) Watch(ch chan<- InotifyEvent) {
	buf := make([]byte, syscall.SizeofInotifyEvent<<12)
	for {
		n, err := f.file.Read(buf)

		switch {
		case errors.Is(err, os.ErrClosed):
			return
		case err != nil:
			if err2, ok := err.(*os.PathError); ok {
				if err2.Op == "read" && err2.Err.Error() == "bad file descriptor" {
					return
				}
			}
			continue
		}

		if n < syscall.SizeofInotifyEvent {
			continue
		}

		var offset int

		for offset <= (n - syscall.SizeofInotifyEvent) {
			var nameBuilder = strings.Builder{}
			p := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))

			if p.Mask&syscall.IN_IGNORED == syscall.IN_IGNORED {
				offset += int(syscall.SizeofInotifyEvent + p.Len)
				continue
			}

			if p.Len > 0 {
				b := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[offset+syscall.SizeofInotifyEvent]))
				for i := 0; i < int(p.Len); i++ {
					if b[i] == 0 {
						break
					}
					nameBuilder.WriteByte(b[i])
				}
			}

			event := newEvent(nameBuilder.String(), f.watchdesc[int(p.Wd)], p)

			select {
			case ch <- event:
			default:
			}

			offset += int(syscall.SizeofInotifyEvent + p.Len)

			log.Debug("inotify:", event.Name, event.Op)
		}
	}
}
