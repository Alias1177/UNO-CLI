package httpR

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"time"
)

func RunHttpTrace(url string) {
	req, _ := http.NewRequest("GET", url, nil)

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			fmt.Println("DNS start:", info.Host)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			fmt.Println("DNS done:", info.Addrs, "Err:", info.Err)
		},
		ConnectStart: func(network, addr string) {
			fmt.Println("Connect start:", network, addr)
		},
		ConnectDone: func(network, addr string, err error) {
			fmt.Println("Connect done:", network, addr, "Err:", err)
		},
		TLSHandshakeStart: func() {
			fmt.Println("TLS handshake start")
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			fmt.Println("TLS handshake done. Err:", err)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			fmt.Println("Got Conn. Reused:", info.Reused, "Was Idle:", info.WasIdle)
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			fmt.Println("Wrote request. Err:", info.Err)
		},
		GotFirstResponseByte: func() {
			fmt.Println("Got first response byte")
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))

	start := time.Now()
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		fmt.Println("Ошибка запроса:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Response status:", resp.Status)
	fmt.Println("Total time:", time.Since(start))
}
