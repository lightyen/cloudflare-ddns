package proc

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var ErrParseUptime = errors.New("parse /proc/uptime error")

type UptimeData struct {
	Uptime float64 // second
	Idle   float64 // second
}

func Uptime() (*UptimeData, error) {
	f, err := os.Open("/proc/uptime")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return nil, ErrParseUptime
	}
	tokens := strings.Fields(scanner.Text())
	if len(tokens) < 2 {
		return nil, ErrParseUptime
	}
	uptime, err := strconv.ParseFloat(tokens[0], 64)
	if err != nil {
		return nil, fmt.Errorf("Invalid /proc/uptime: %w", err)
	}
	idle, err := strconv.ParseFloat(tokens[1], 64)
	if err != nil {
		return nil, fmt.Errorf("Invalid /proc/uptime: %w", err)
	}
	var u UptimeData
	u.Uptime = uptime
	u.Idle = idle
	return &u, nil
}
