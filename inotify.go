package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

var (
	ErrWatched = errors.New("already watched")
)

type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func flagMask[T Unsigned](mask T, v T) bool {
	return mask&v == v
}

func (o Op) String() string {
	s := new(strings.Builder)

	if flagMask(o, Create) {
		s.WriteString("|Create")
	}
	if flagMask(o, Remove) {
		s.WriteString("|Remove")
	}
	if flagMask(o, Rename) {
		s.WriteString("|Rename")
	}
	if flagMask(o, CloseWrite) {
		s.WriteString("|CloseWrite")
	}
	if flagMask(o, Modify) {
		s.WriteString("|Write")
	}
	if flagMask(o, Chmod) {
		s.WriteString("|Chmod")
	}
	if s.Len() == 0 {
		return fmt.Sprintf("Undefined(0x%04X)", uint32(o))
	}
	return s.String()[1:]
}

type Mask uint32

func (m Mask) String() string {
	s := new(strings.Builder)

	if flagMask(m, syscall.IN_CREATE) {
		s.WriteString("|IN_CREATE")
	}
	if flagMask(m, syscall.IN_DELETE) {
		s.WriteString("|IN_DELETE")
	}
	if flagMask(m, syscall.IN_DELETE_SELF) {
		s.WriteString("|IN_DELETE_SELF")
	}
	if flagMask(m, syscall.IN_MOVE_SELF) {
		s.WriteString("|IN_MOVE_SELF")
	}
	if flagMask(m, syscall.IN_MOVED_TO) {
		s.WriteString("|IN_MOVED_TO")
	}
	if flagMask(m, syscall.IN_MOVED_FROM) {
		s.WriteString("|IN_MOVED_FROM")
	}
	if flagMask(m, syscall.IN_CLOSE_WRITE) {
		s.WriteString("|IN_CLOSE_WRITE")
	}
	if flagMask(m, syscall.IN_CLOSE_NOWRITE) {
		s.WriteString("|IN_CLOSE_NOWRITE")
	}
	if flagMask(m, syscall.IN_MODIFY) {
		s.WriteString("|IN_MODIFY")
	}
	if flagMask(m, syscall.IN_ACCESS) {
		s.WriteString("|IN_ACCESS")
	}
	if flagMask(m, syscall.IN_ATTRIB) {
		s.WriteString("|IN_ATTRIB")
	}

	if flagMask(m, syscall.IN_IGNORED) {
		s.WriteString("|IN_IGNORED")
	}
	if flagMask(m, syscall.IN_ISDIR) {
		s.WriteString("|IN_ISDIR")
	}
	if flagMask(m, syscall.IN_Q_OVERFLOW) {
		s.WriteString("|IN_Q_OVERFLOW")
	}
	if flagMask(m, syscall.IN_UNMOUNT) {
		s.WriteString("|IN_UNMOUNT")
	}

	if s.Len() == 0 {
		return fmt.Sprintf("Undefined(%d)", m)
	}

	return s.String()[1:]
}

func maskToOp(mask uint32) (op Op) {
	if flagMask(mask, syscall.IN_CREATE) || flagMask(mask, syscall.IN_MOVED_TO) {
		op |= Create
	}
	if flagMask(mask, syscall.IN_DELETE_SELF) || flagMask(mask, syscall.IN_DELETE) {
		op |= Remove
	}
	if flagMask(mask, syscall.IN_MOVE_SELF) || flagMask(mask, syscall.IN_MOVED_FROM) {
		op |= Rename
	}
	if flagMask(mask, syscall.IN_CLOSE_WRITE) {
		op |= CloseWrite
	}
	if flagMask(mask, syscall.IN_MODIFY) {
		op |= Modify
	}
	if flagMask(mask, syscall.IN_ATTRIB) {
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
	mu      sync.RWMutex
	wdDir   map[int]string
	dirWd   map[string]int
	targets map[string]int
}

func (w *watches) getDir(e *syscall.InotifyEvent) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.wdDir[int(e.Wd)]
}

func (w *watches) deleteSelf(e *syscall.InotifyEvent) (ok bool) {
	wd := int(e.Wd)
	w.mu.Lock()
	defer w.mu.Unlock()
	var dir string
	dir, ok = w.wdDir[wd]
	if !ok {
		return
	}
	delete(w.wdDir, wd)
	delete(w.dirWd, dir)
	for t, fd := range w.targets {
		if fd == wd {
			delete(w.targets, t)
		}
	}
	return
}

func (w *watches) watched() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var s []string
	for k := range w.targets {
		s = append(s, k)
	}
	return s
}

func NewINotify() *INotify {
	return &INotify{
		watches: &watches{
			wdDir:   map[int]string{},
			dirWd:   map[string]int{},
			targets: map[string]int{},
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
	for w := range f.watches.wdDir {
		syscall.InotifyRmWatch(f.fd, uint32(w))
	}
	clear(f.watches.wdDir)
	clear(f.watches.dirWd)
	clear(f.watches.targets)
	return f.file.Close()
}

func (f *INotify) Watched() []string {
	return f.watches.watched()
}

func (f *INotify) AddWatch(path string, op Op) error {
	f.watches.mu.Lock()
	defer f.watches.mu.Unlock()

	t := filepath.Clean(path)
	if _, exists := f.watches.targets[t]; exists {
		return ErrWatched
	}

	dir := filepath.Dir(t)

	wd, exists := f.watches.dirWd[dir]
	if exists {
		f.watches.targets[t] = wd
		return nil
	}

	wd, err := syscall.InotifyAddWatch(f.fd, dir, uint32(op))
	if err != nil {
		return err
	}

	f.watches.targets[t] = wd
	f.watches.dirWd[dir] = wd
	f.watches.wdDir[wd] = dir
	return nil
}

func (f *INotify) Watch(ch chan<- InotifyEvent) error {
	buf := make([]byte, syscall.SizeofInotifyEvent<<12)
	for {
		n, err := f.file.Read(buf)

		if errors.Is(err, os.ErrClosed) {
			return err
		}

		if err != nil {
			if err2, ok := err.(*os.PathError); ok {
				if err2.Op == "read" && err2.Err.Error() == "bad file descriptor" {
					return err
				}
			}
			return err
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

			event := InotifyEvent{
				Len:  e.Len,
				Mask: Mask(e.Mask),
				Name: s.String(),
				Path: f.watches.getDir(e),
				Op:   maskToOp(e.Mask),
			}

			if e.Mask&syscall.IN_DELETE_SELF == syscall.IN_DELETE_SELF {
				f.watches.deleteSelf(e)
			}

			t := filepath.Clean(filepath.Join(event.Path, event.Name))

			_, exists := f.watches.targets[t]
			if exists {
				ch <- event
			}

			offset += int(syscall.SizeofInotifyEvent + e.Len)
		}
	}
}
