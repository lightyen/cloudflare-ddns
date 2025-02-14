package log

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	defaultTimeFormat = "2006-01-02T15-04-05.000"
	compressSuffix    = ".zst"  // https://github.com/facebook/zstd
	defaultMaxSize    = 8 << 20 // 8 MiB
)

type LogrotateOption struct {
	// Filename is the file to write logs to.  Backup log files will be retained
	// in the same directory.  It uses <processname>-lumberjack.log in
	// os.TempDir() if empty.
	Filename string

	// MaxSize is the maximum size in bytes of the log file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxSize int

	// MaxBackups is the maximum number of old log files to retain.  The default
	// is to retain all old log files (though MaxAge may still cause them to get
	// deleted.)
	MaxBackups int

	MaxAge time.Duration

	BackupTimeFormat string

	OnMillFailed func(error)

	// File header to be written for each new log file created
	FileHeader string

	// Compress determines if the rotated log files should be compressed
	// using gzip. The default is not to perform compression.
	Compress bool
}

type LogrotateWriter struct {
	filename string
	dirname  string
	basename string
	ext      string

	options LogrotateOption
	mu      sync.Mutex
	size    int64
	file    *os.File

	mu2 sync.Mutex
}

func NewLogrotateWriter(options LogrotateOption) *LogrotateWriter {
	l := &LogrotateWriter{options: options}
	if l.options.BackupTimeFormat == "" {
		l.options.BackupTimeFormat = defaultTimeFormat
	}

	if l.options.Filename != "" {
		l.filename = filepath.Clean(l.options.Filename)
	} else {
		l.filename = filepath.Join(os.TempDir(), filepath.Base(os.Args[0])+"-messages.log")
	}

	l.dirname, l.basename = filepath.Split(l.filename)
	l.ext = filepath.Ext(l.filename)

	if l.options.MaxSize <= 0 {
		l.options.MaxSize = defaultMaxSize
	}

	return l
}

func (l *LogrotateWriter) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.write(p)
}

func (l *LogrotateWriter) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.close()
}

func (l *LogrotateWriter) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rotate()
}

func (l *LogrotateWriter) openExistingOrNew(writeLen int) error {
	fi, err := os.Stat(l.filename)
	if errors.Is(err, fs.ErrNotExist) {
		return l.openNew()
	}

	if err != nil {
		return fmt.Errorf("error getting log file info: %w", err)
	}

	if fi.Size()+int64(writeLen) >= l.max() {
		return l.rotate()
	}

	file, err := os.OpenFile(l.filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return l.openNew()
	}

	l.file = file
	l.size = fi.Size()
	return nil
}

func (l *LogrotateWriter) write(p []byte) (n int, err error) {
	writeLen := int64(len(p))
	if writeLen > l.max() {
		return 0, fmt.Errorf(
			"write length %d exceeds maximum file size %d", writeLen, l.max(),
		)
	}

	if l.file == nil {
		if err = l.openExistingOrNew(len(p)); err != nil {
			return 0, err
		}
	}

	if l.size+writeLen > l.max() {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.size += int64(n)

	return n, err
}

func (l *LogrotateWriter) close() error {
	if l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}

func (l *LogrotateWriter) rotate() error {
	if err := l.close(); err != nil {
		return err
	}
	if err := l.openNew(); err != nil {
		return err
	}
	go l.TryMill()
	return nil
}

func (l *LogrotateWriter) TryMill() {
	ok := l.mu2.TryLock()
	if !ok {
		// goruntine is busy.
		return
	}
	defer l.mu2.Unlock()
	if err := l.mill(); err != nil {
		if l.options.OnMillFailed != nil {
			l.options.OnMillFailed(err)
		}
	}
}

func (l *LogrotateWriter) timeFromName(filename string, prefix, ext string) (time.Time, bool) {
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, false
	}

	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, false
	}

	ts := filename[len(prefix) : len(filename)-len(ext)]

	t, err := time.Parse(l.options.BackupTimeFormat, ts)
	return t, err == nil
}

func (l *LogrotateWriter) max() int64 {
	return int64(l.options.MaxSize)
}

type logInfo struct {
	t    time.Time
	name string
}

func (l *LogrotateWriter) oldLogFiles() ([]logInfo, error) {
	entries, err := fs.ReadDir(os.DirFS(l.dirname), ".")
	if err != nil {
		return nil, fmt.Errorf("can't read log file directory: %w", err)
	}

	prefix := l.basename[:len(l.basename)-len(l.ext)] + "-"

	var oldFiles []logInfo

	for _, f := range entries {
		if !f.Type().IsRegular() {
			continue
		}

		if t, ok := l.timeFromName(f.Name(), prefix, l.ext); ok {
			oldFiles = append(oldFiles, logInfo{t, f.Name()})
			continue
		}

		if t, ok := l.timeFromName(f.Name(), prefix, l.ext+compressSuffix); ok {
			oldFiles = append(oldFiles, logInfo{t, f.Name()})
			continue
		}
	}

	slices.SortFunc(oldFiles, func(a logInfo, b logInfo) int {
		return b.t.Compare(a.t)
	})

	return oldFiles, nil
}

func (l *LogrotateWriter) backupName() string {
	s := l.basename
	prefix := s[:len(s)-len(l.ext)] + "-"

	timestamp := time.Now().UTC().Format(l.options.BackupTimeFormat)

	return filepath.Join(l.dirname, fmt.Sprintf("%s%s%s", prefix, timestamp, l.ext))
}

func (l *LogrotateWriter) openNew() error {
	if l.dirname != "" {
		err := os.MkdirAll(l.dirname, 0755)
		if err != nil {
			return fmt.Errorf("can't make directories for new logfile: %w", err)
		}
	}

	mode := os.FileMode(0644)
	fi, err := os.Stat(l.filename)

	if err == nil {
		mode = fi.Mode()
		if err := os.Rename(l.filename, l.backupName()); err != nil {
			return fmt.Errorf("can't rename log file: %w", err)
		}
	}

	f, err := os.OpenFile(l.filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("can't open new logfile: %w", err)
	}

	l.file = f
	l.size = 0

	if l.options.FileHeader != "" {
		n, err := l.file.WriteString(l.options.FileHeader)
		if err != nil {
			return fmt.Errorf("can't write file header to file: %w", err)
		}
		l.size += int64(n)
	}
	return nil
}

func (l *LogrotateWriter) mill() error {
	if l.options.MaxBackups == 0 && l.options.MaxAge == 0 && !l.options.Compress {
		return nil
	}

	files, err := l.oldLogFiles()
	if err != nil {
		return err
	}

	var remove []logInfo

	if l.options.MaxBackups > 0 && l.options.MaxBackups < len(files) {
		preserved := make(map[string]struct{})
		var remaining []logInfo
		for _, f := range files {
			fn := f.name
			fn = strings.TrimSuffix(fn, compressSuffix)
			preserved[fn] = struct{}{}

			if len(preserved) > l.options.MaxBackups {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}

	if l.options.MaxAge > 0 {
		cutoff := time.Now().Add(-l.options.MaxAge)

		var remaining []logInfo
		for _, f := range files {
			if f.t.Before(cutoff) {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}

	var compress []logInfo

	if l.options.Compress {
		for _, f := range files {
			if !strings.HasSuffix(f.name, compressSuffix) {
				compress = append(compress, f)
			}
		}
	}

	for _, f := range remove {
		errRemove := os.Remove(filepath.Join(l.dirname, f.name))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}

	for _, f := range compress {
		errCompress := l.compressFile(f.name)
		if err == nil && errCompress != nil {
			err = errCompress
		}
	}

	return err
}

func (l *LogrotateWriter) compressFile(name string) (err error) {
	src := filepath.Join(l.dirname, name)
	dst := filepath.Join(l.dirname, name+compressSuffix)

	defer func() {
		if err != nil {
			_ = os.Remove(dst)
			err = fmt.Errorf("compressFile: %w", err)
			return
		}
		if err2 := os.Remove(src); err2 != nil {
			err = err2
		}
	}()

	fi, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if err2 := f.Close(); err == nil && err2 != nil {
			err = err2
		}
	}()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode())
	if err != nil {
		return fmt.Errorf("failed to open compressed log file: %w", err)
	}
	defer func() {
		if err2 := out.Close(); err == nil && err2 != nil {
			err = err2
		}
	}()

	enc, err := zstd.NewWriter(out)
	if err != nil {
		return err
	}

	defer func() {
		if err2 := enc.Close(); err == nil && err2 != nil {
			err = err2
		}
	}()

	_, err = io.Copy(enc, f)
	return
}
