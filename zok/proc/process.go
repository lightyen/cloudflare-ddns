package proc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func FindProcess(nameOrPID string) (int, []byte, bool) {
	var exists = false
	var pID int
	var status []byte
	_ = filepath.WalkDir("/proc", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dirname := d.Name()
		if dirname == "proc" {
			return nil
		}

		pID, _ = strconv.Atoi(dirname)
		if pID <= 0 {
			return filepath.SkipDir
		}

		data, err := os.ReadFile(path + "/status")
		if err != nil {
			return filepath.SkipDir
		}

		var pName string
		n := bytes.IndexByte(data, '\n')
		if n == -1 {
			pName = strings.TrimSpace(string(data[6:]))
		} else {
			pName = strings.TrimSpace(string(data[6:n]))
		}

		if nameOrPID != pName && nameOrPID != dirname {
			return filepath.SkipDir
		}

		p, err := os.FindProcess(pID)
		if err != nil {
			return filepath.SkipAll
		}
		if p != nil {
			exists = true
			status = data
			return filepath.SkipAll
		}
		return filepath.SkipDir
	})
	if !exists {
		pID = 0
		status = nil
	}
	return pID, status, exists
}

type ProcStatus struct {
	Name   string
	Pid    int
	VMPeak uint64 // Peak virtual memory size(KB)
	VMSize uint64 // Virtual memory size(KB)
	VMRss  uint64 // Resident set size(KB)
}

func SelfStatus() (*ProcStatus, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", syscall.Getpid()))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseProcessStatus(f)
}

func parseProcessStatus(r io.Reader) (*ProcStatus, error) {
	scanner := bufio.NewScanner(r)
	var m ProcStatus
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "Name:":
			m.Name = fields[1]
			continue
		case "Pid:":
			var err error
			m.Pid, err = strconv.Atoi(fields[1])
			if err != nil {
				return nil, err
			}
			continue
		case "VmPeak:":
		case "VmSize:":
		case "VmRSS:":
		default:
			continue
		}
		v, err := strconv.ParseUint(fields[1], 0, 64)
		if err != nil {
			return nil, err
		}
		switch fields[0] {
		case "VmPeak:":
			m.VMPeak = v
		case "VmSize:":
			m.VMSize = v
		case "VmRSS:":
			m.VMRss = v
		}
	}
	return &m, nil
}

type ProcessStat struct {
	Pid              int
	Filename         string
	State            string
	PPid             int
	UserTime         float64
	SysTime          float64
	ChildrenUserTime float64
	ChildrenSysTime  float64
}

// get process stat
func PStat(pid int) (*ProcessStat, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	if scanner.Scan() {
		line := scanner.Text()
		var stat ProcessStat
		var _p [9]int
		count, err := fmt.Sscanf(line, "%d %s %s %d %d %d %d %d %d %d %d %d %d %f %f %f %f",
			&stat.Pid, &stat.Filename, &stat.State, &stat.PPid,
			&_p[0], &_p[1], &_p[2], &_p[3], &_p[4], &_p[5], &_p[6], &_p[7], &_p[8],
			&stat.UserTime,
			&stat.SysTime,
			&stat.ChildrenUserTime,
			&stat.ChildrenSysTime,
		)
		if err == nil && count > 10 {
			return &stat, nil
		}
	}
	return nil, fmt.Errorf("parse /proc/%d/stat", pid)
}
