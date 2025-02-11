package server

import (
	"net"
)

func OutboundIPv6() (string, error) {
	conn, err := net.Dial("udp6", "[2606:4700:4700::1111]:53")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
