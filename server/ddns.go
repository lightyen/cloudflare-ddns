package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

// curl 'https://api.ipify.org'
// curl -6 'https://api64.ipify.org'
// https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/list/

var (
	client  = &http.Client{}
	client4 = &http.Client{}
	client6 = &http.Client{}
)

func init() {
	dial := &net.Dialer{}
	{
		t := &http.Transport{}
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dial.DialContext(ctx, "tcp4", addr)
		}
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client4.Transport = t
		client4.Timeout = 30 * time.Second
	}
	{
		t := &http.Transport{}
		t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dial.DialContext(ctx, "tcp6", addr)
		}
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client6.Transport = t
		client6.Timeout = 30 * time.Second
	}
}

type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CloudflareRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Proxiable bool   `json:"proxiable"`
	Proxied   bool   `json:"proxied"`
	Type      string `json:"type"`
}

func (s *Server) ddns(ctx context.Context) {
	go func() {
		for {
			s.apply <- struct{}{}
			time.Sleep(15 * time.Minute)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.apply:
			if err := s.modify(); err != nil {
				log.Error(err)
			}
		}
	}
}

func GetInternetAddrs() (ipv4, ipv6 string, err error) {
	// curl 'https://api.ipify.org'
	// curl -6 'https://api64.ipify.org'
	type Response struct {
		Content string `json:"ip"`
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		var v Response
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := request(client4, ctx, "GET", "https://api.ipify.org?format=json", &v, nil); err != nil {
			// nothing
		}
		ipv4 = v.Content
	}()

	go func() {
		defer wg.Done()
		if config.Config().StaticIPv6 != "" {
			ipv6 = config.Config().StaticIPv6
			return
		}
		ipv6, _ = OutboundIPv6()
	}()

	wg.Wait()

	if ipv4 == "" && ipv6 == "" {
		return "", "", errors.New("Get Internet IPs failed.")
	}

	return
}

func (s *Server) modify() error {
	ipv4, ipv6, err := GetInternetAddrs()
	if err != nil {
		return err
	}

	if ipv6 == "" {
		log.Warn("Internet v6 not found.")
	}

	records, err := getRecords()
	if err != nil {
		return err
	}

	// add not exists
	for _, rule := range config.Config().Records {
		var exists bool
		for _, r := range records {
			if r.Name == rule.Name && r.Type == rule.Type {
				exists = true
				break
			}
		}

		if exists {
			continue
		}

		switch rule.Type {
		case "A":
			if ipv4 != "" {
				if err := addRecord(rule.Name, rule.Type, ipv4); err != nil {
					log.Error(err)
				} else {
					log.Infof("ADD record: {%s %s: %s}", rule.Type, rule.Name, ipv4)
				}
			}
		case "AAAA":
			if ipv6 != "" {
				if err := addRecord(rule.Name, rule.Type, ipv6); err != nil {
					log.Error(err)
				} else {
					log.Infof("ADD record: {%s %s: %s}", rule.Type, rule.Name, ipv6)
				}
			}
		}
	}

	for _, r := range records {
		var exists bool
		for _, v := range config.Config().Records {
			if r.Name == v.Name && r.Type == v.Type {
				exists = true
				break
			}
		}

		// delete if not found
		if !exists {
			if err := deleteRecord(r.ID); err != nil {
				log.Error(err)
			} else {
				log.Infof("DELETE record: {%s %s: %s}", r.Type, r.Name, r.Content)
			}
			continue
		}

		// patch
		switch r.Type {
		case "A":
			if ipv4 != "" && r.Content != ipv4 {
				if err := patchRecord(r, ipv4); err != nil {
					log.Error(err)
				} else {
					log.Infof("PATCH record: {%s %s: %s}", r.Type, r.Name, ipv4)
				}
			}
		case "AAAA":
			if ipv6 != "" && r.Content != ipv6 {
				if err := patchRecord(r, ipv6); err != nil {
					log.Error(err)
				} else {
					log.Infof("PATCH record: {%s %s: %s}", r.Type, r.Name, ipv6)
				}
			}
		}
	}

	return nil
}

func RequestCloudflare(ctx context.Context, method, path string, body io.Reader, resData any) error {
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", config.Config().ZoneID)+path, body)
	if err != nil {
		return err
	}

	req.Header.Set("X-Auth-Email", config.Config().Email)
	req.Header.Set("X-Auth-Key", config.Config().Token)

	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}

	res, err := client.Do(req)
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, res.Body); err != nil {
		return err
	}

	///

	var result struct {
		Errors  []CloudflareError `json:"errors"`
		Success bool              `json:"success"`
	}

	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return err
	}

	if result.Success {
		if resData == nil {
			return nil
		}
		return json.Unmarshal(buf.Bytes(), resData)
	}

	if len(result.Errors) > 0 && result.Errors[0].Message != "" {
		return fmt.Errorf("cloudflare: %s %s %s", result.Errors[0].Message, req.Method, req.URL)
	}

	return fmt.Errorf("cloudflare: %s %s %s", "Unknown Error", req.Method, req.URL)
}

func request(client *http.Client, ctx context.Context, method, url string, resData any, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	res, err := client.Do(req)
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	if err != nil {
		return err
	}

	if resData == nil {
		return nil
	}

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, res.Body); err != nil {
		return err
	}

	return json.Unmarshal(buf.Bytes(), resData)
}

func addRecord(fullName string, typ string, ip string) error {
	type Record struct {
		Name    string `json:"name"`
		Content string `json:"content"`
		Type    string `json:"type"`
		Proxied bool   `json:"proxied"`
	}

	b, err := json.Marshal(Record{
		Name:    fullName,
		Type:    typ,
		Content: ip,
		Proxied: false,
	})
	if err != nil {
		return err
	}

	return RequestCloudflare(context.Background(), "POST", "/dns_records", bytes.NewBuffer(b), nil)
}

func patchRecord(record CloudflareRecord, ip string) error {
	type Record struct {
		Name    string `json:"name"`
		Content string `json:"content"`
		Type    string `json:"type"`
		Proxied bool   `json:"proxied"`
	}

	b, err := json.Marshal(Record{
		Name:    record.Name,
		Type:    record.Type,
		Content: ip,
		Proxied: false,
	})
	if err != nil {
		return err
	}

	return RequestCloudflare(context.Background(), "PATCH", "/dns_records/"+record.ID, bytes.NewBuffer(b), nil)
}

func deleteRecord(id string) error {
	return RequestCloudflare(context.Background(), "DELETE", fmt.Sprintf("/dns_records/%s", id), nil, nil)
}

func getRecords() ([]CloudflareRecord, error) {
	var result struct {
		Data    []CloudflareRecord `json:"result"`
		Errors  []CloudflareError  `json:"errors"`
		Success bool               `json:"success"`
	}
	if err := RequestCloudflare(context.Background(), "GET", "/dns_records", nil, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}
