package server

import (
	"crypto/tls"
	"os"

	"golang.org/x/crypto/pkcs12"
)

func X509Pfx(pfxFile string, passphrase string) (func(*tls.ClientHelloInfo) (*tls.Certificate, error), error) {
	data, err := os.ReadFile(pfxFile)
	if err != nil {
		return nil, err
	}

	var c tls.Certificate
	key, cert, err := pkcs12.Decode(data, passphrase)
	if err != nil {
		return nil, err
	}

	c.PrivateKey = key
	c.Certificate = [][]byte{cert.Raw}

	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return &c, nil
	}, nil
}

func X509KeyPair(certFile string, keyFile string) (func(*tls.ClientHelloInfo) (*tls.Certificate, error), error) {
	c, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return &c, nil
	}, nil
}
