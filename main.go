// package throttle_proxy
package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

var domainRateConfig = map[string]RateConfig{
	"example.com": {Limit: 5, Period: "second"},
	"another.com": {Limit: 30, Period: "minute"},
}

// RequestWrapper Struct to wrap the request and response writer together
type RequestWrapper struct {
	w http.ResponseWriter
	r *http.Request
}

// RateConfig Configuration structure for rate limiting
type RateConfig struct {
	Limit  int
	Period string
}

// A map to hold domain-specific queues
var domainQueues = make(map[string]chan RequestWrapper)

func main() {
	// Initialize domain-specific queues and goroutines
	//for domain, config := range domainRateConfig {
	//	var limiter *rate.Limiter
	//	if config.Period == "second" {
	//		limiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(config.Limit)), config.Limit)
	//	} else if config.Period == "minute" {
	//		limiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(config.Limit)), config.Limit)
	//	}
	//
	//	queue := make(chan RequestWrapper, 1000) // buffer size of 1000 per domain
	//	domainQueues[domain] = queue
	//
	//	go processDomainQueue(domain, queue, limiter)
	//}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			handleConnect(w, r)
		} else {
			handleRequest(w, r) // inside this function, call the appropriate handler, likely proxyHandler
		}
	})

	//http.Handle("/", loggingMiddleware(http.HandlerFunc(handleRequest)))
	log.Println("Server started and listening on 0.0.0.0:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received %s request from %s to %s", r.Method, r.RemoteAddr, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	//domain := r.Host // Extract the domain from the request Host header
	//if queue, exists := domainQueues[domain]; exists {
	//	// If domain is in the config, enqueue for rate limiting
	//	queue <- RequestWrapper{w: w, r: r}
	//} else {
	//	// If domain isn't in the config, process immediately
	//	if r.Method == http.MethodConnect {
	//		handleConnect(w, r)
	//	} else {
	//		proxyHandler(w, r)
	//	}
	//}
	log.Println("handleRequest()")
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

//func processDomainQueue(domain string, queue chan RequestWrapper, limiter *rate.Limiter) {
//	for reqWrapper := range queue {
//		if limiter.Wait(context.Background()) == nil {
//			if reqWrapper.r.Method == http.MethodConnect {
//				handleConnect(reqWrapper.w, reqWrapper.r)
//			} else {
//				proxyHandler(reqWrapper.w, reqWrapper.r)
//			}
//		}
//	}
//}

// handleConnect manages the CONNECT method for HTTPS tunneling
func handleConnect(w http.ResponseWriter, r *http.Request) {
	log.Println("handleConnect()")
	destConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		//clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		//clientConn.Close()
		return
	}

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack the connection", http.StatusServiceUnavailable)
		return
	}

	//// Inform the client that a connection has been established
	//clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Step 3: Relay raw bytes between the client and the target server.
	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

// proxyHandler manages non-CONNECT HTTP requests
//func proxyHandler(w http.ResponseWriter, r *http.Request) {
//	log.Printf("Proxying request: %s %s", r.Method, r.URL.String())
//	// Create a new request based on the original. This copies the method, URL, header, etc.
//	req, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
//	if err != nil {
//		http.Error(w, "Failed to create a new request", http.StatusInternalServerError)
//		return
//	}
//	req.Header = r.Header
//
//	// Send the request to the destination
//	resp, err := http.DefaultClient.Do(req)
//	if err != nil {
//		http.Error(w, "Failed to reach the destination", http.StatusServiceUnavailable)
//		return
//	}
//	defer resp.Body.Close()
//
//	// Copy headers and status code from the response
//	for key, values := range resp.Header {
//		for _, value := range values {
//			w.Header().Add(key, value)
//		}
//	}
//	w.WriteHeader(resp.StatusCode)
//
//	// Copy the response body
//	io.Copy(w, resp.Body)
//}

// transfer manages data transfer between two connections
func transfer(dst net.Conn, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}
