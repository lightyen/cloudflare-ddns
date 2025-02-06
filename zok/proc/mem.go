package proc

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

type Memmoryinfo struct {
	MemTotal     uint64
	MemFree      uint64
	MemAvailable uint64
}

var ErrParseProcMemInfo = errors.New("parse /proc/meminfo error")

// Show memory information
func Memory() (*Memmoryinfo, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var m Memmoryinfo
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			return nil, ErrParseProcMemInfo
		}
		v, err := strconv.ParseUint(fields[1], 0, 64)
		if err != nil {
			return nil, err
		}
		switch fields[0] {
		case "MemTotal:":
			m.MemTotal = v
		case "MemFree:":
			m.MemFree = v
		case "MemAvailable:":
			m.MemAvailable = v
		}
	}
	return &m, nil
}
