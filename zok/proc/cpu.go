package proc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type CPUStat struct {
	User    float64
	Nice    float64
	System  float64
	Idle    float64
	Iowait  float64
	IRQ     float64
	SoftIRQ float64

	Steal     float64
	Guest     float64
	GuestNice float64
}

var ErrParseProcStat = errors.New("parse /proc/stat error")

func (c *CPUStat) UserTime() float64 {
	return c.User - c.Guest
}

func (c *CPUStat) NiceTime() float64 {
	return c.Nice - c.GuestNice
}

func (c *CPUStat) IdleTime() float64 {
	return c.Idle + c.Iowait
}

func (c *CPUStat) SystemTime() float64 {
	return c.System + c.IRQ + c.SoftIRQ
}

func (c *CPUStat) VirtualTime() float64 {
	return c.Guest + c.GuestNice
}

func (c *CPUStat) TotalTime() float64 {
	return c.UserTime() + c.NiceTime() + c.SystemTime() + c.IdleTime() + c.VirtualTime() + c.Steal
}

func Stat() (map[string]CPUStat, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	ret := make(map[string]CPUStat)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu") {
			var cpu string
			var stat CPUStat
			count, err := fmt.Sscanf(line, "%s %f %f %f %f %f %f %f %f %f %f",
				&cpu,
				&stat.User, &stat.Nice, &stat.System, &stat.Idle,
				&stat.Iowait, &stat.IRQ, &stat.SoftIRQ,
				&stat.Steal, &stat.Guest, &stat.GuestNice)
			if err != nil && err != io.EOF {
				return nil, ErrParseProcStat
			}
			if count == 0 {
				return nil, ErrParseProcStat
			}
			ret[cpu] = stat
		}
	}
	if len(ret) == 0 {
		return nil, ErrParseProcStat
	}
	return ret, nil
}
