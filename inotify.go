package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type Op uint32

const (
	Create     Op = syscall.IN_CREATE | syscall.IN_MOVED_TO
	Remove     Op = syscall.IN_DELETE | syscall.IN_DELETE_SELF
	Rename     Op = syscall.IN_MOVED_FROM | syscall.IN_MOVE_SELF
	CloseWrite Op = syscall.IN_CLOSE_WRITE
	Modify     Op = syscall.IN_MODIFY
	Chmod      Op = syscall.IN_ATTRIB
)

type InotifyEvent struct {
	Len  uint32
	Mask Mask
	Name string
	Path string
	Op   Op
}

type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func flag[T Unsigned](mask T, v T) bool {
	return mask&v == v
}

func (o Op) String() string {
	var b = &strings.Builder{}

	if flag(o, Create) {
		b.WriteString("|Create")
	}
	if flag(o, Remove) {
		b.WriteString("|Remove")
	}
	if flag(o, Rename) {
		b.WriteString("|Rename")
	}
	if flag(o, CloseWrite) {
		b.WriteString("|CloseWrite")
	}
	if flag(o, Modify) {
		b.WriteString("|Write")
	}
	if flag(o, Chmod) {
		b.WriteString("|Chmod")
	}

	if b.Len() == 0 {
		return fmt.Sprintf("Undefined(%d)", o)
	}

	return b.String()[1:]
}

type Mask uint32

func (m Mask) String() string {
	var b = &strings.Builder{}

	if flag(m, syscall.IN_CREATE) {
		b.WriteString("|IN_CREATE")
	}
	if flag(m, syscall.IN_DELETE) {
		b.WriteString("|IN_DELETE")
	}
	if flag(m, syscall.IN_DELETE_SELF) {
		b.WriteString("|IN_DELETE_SELF")
	}
	if flag(m, syscall.IN_MOVE_SELF) {
		b.WriteString("|IN_MOVE_SELF")
	}
	if flag(m, syscall.IN_MOVED_TO) {
		b.WriteString("|IN_MOVED_TO")
	}
	if flag(m, syscall.IN_MOVED_FROM) {
		b.WriteString("|IN_MOVED_FROM")
	}
	if flag(m, syscall.IN_CLOSE_WRITE) {
		b.WriteString("|IN_CLOSE_WRITE")
	}
	if flag(m, syscall.IN_CLOSE_NOWRITE) {
		b.WriteString("|IN_CLOSE_NOWRITE")
	}
	if flag(m, syscall.IN_MODIFY) {
		b.WriteString("|IN_MODIFY")
	}
	if flag(m, syscall.IN_ACCESS) {
		b.WriteString("|IN_ACCESS")
	}
	if flag(m, syscall.IN_ATTRIB) {
		b.WriteString("|IN_ATTRIB")
	}

	if flag(m, syscall.IN_IGNORED) {
		b.WriteString("|IN_IGNORED")
	}
	if flag(m, syscall.IN_ISDIR) {
		b.WriteString("|IN_ISDIR")
	}
	if flag(m, syscall.IN_Q_OVERFLOW) {
		b.WriteString("|IN_Q_OVERFLOW")
	}
	if flag(m, syscall.IN_UNMOUNT) {
		b.WriteString("|IN_UNMOUNT")
	}

	if b.Len() == 0 {
		return fmt.Sprintf("Undefined(%d)", m)
	}

	return b.String()[1:]
}

func maskToOp(mask uint32) (op Op) {
	if flag(mask, syscall.IN_CREATE) || flag(mask, syscall.IN_MOVED_TO) {
		op |= Create
	}
	if flag(mask, syscall.IN_DELETE_SELF) || flag(mask, syscall.IN_DELETE) {
		op |= Remove
	}
	if flag(mask, syscall.IN_MOVE_SELF) || flag(mask, syscall.IN_MOVED_FROM) {
		op |= Rename
	}
	if flag(mask, syscall.IN_CLOSE_WRITE) {
		op |= CloseWrite
	}
	if flag(mask, syscall.IN_MODIFY) {
		op |= Modify
	}
	if flag(mask, syscall.IN_ATTRIB) {
		op |= Chmod
	}

	return
}

type INotify struct {
	fd      int
	file    *os.File
	watches *watches
}

type watches struct {
	mu    sync.RWMutex
	paths map[int]string
	wd    map[string]int
}

func NewINotify() *INotify {
	return &INotify{
		watches: &watches{
			paths: map[int]string{},
			wd:    map[string]int{},
		},
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
	f.watches.mu.Lock()
	defer f.watches.mu.Unlock()
	for w := range f.watches.paths {
		syscall.InotifyRmWatch(f.fd, uint32(w))
	}
	f.watches.paths = map[int]string{}
	f.watches.wd = map[string]int{}
	return f.file.Close()
}

func (f *INotify) AddWatch(path string, op Op) error {
	f.watches.mu.Lock()
	defer f.watches.mu.Unlock()
	_, exists := f.watches.wd[path]
	if exists {
		return nil
	}

	w, err := syscall.InotifyAddWatch(f.fd, path, uint32(op))
	if err != nil {
		return err
	}
	f.watches.wd[path] = w
	f.watches.paths[w] = path
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
			s := bytes.NewBuffer(make([]byte, 0, syscall.PathMax))
			e := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))

			if e.Mask&syscall.IN_IGNORED == syscall.IN_IGNORED {
				offset += int(syscall.SizeofInotifyEvent + e.Len)
				continue
			}

			if e.Len > 0 {
				b := (*[syscall.PathMax]byte)(unsafe.Pointer(&buf[offset+syscall.SizeofInotifyEvent]))
				for i := 0; i < int(e.Len); i++ {
					if b[i] == 0 {
						break
					}
					s.WriteByte(b[i])
				}
			}

			f.watches.mu.RLock()
			event := InotifyEvent{
				Len:  e.Len,
				Mask: Mask(e.Mask),
				Name: s.String(),
				Path: f.watches.paths[int(e.Wd)],
				Op:   maskToOp(e.Mask),
			}
			f.watches.mu.RUnlock()

			if e.Mask&syscall.IN_DELETE_SELF == syscall.IN_DELETE_SELF {
				f.watches.mu.Lock()
				if path, ok := f.watches.paths[int(e.Wd)]; ok {
					delete(f.watches.paths, int(e.Wd))
					delete(f.watches.wd, path)
				}
				f.watches.mu.Unlock()
			}

			select {
			case ch <- event:
			default:
			}

			offset += int(syscall.SizeofInotifyEvent + e.Len)
		}
	}
}
