package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type HttpResponse struct {
	Status int         `json:"status"`
	Os     string      `json:"os"`
	Debug  interface{} `json:"debug,omitempty"`
}

// checkPort tries to connect to the specified host and port to see if it's open
func checkPort(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	timeout := time.Second * 2

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkDNS performs a DNS lookup using either the custom or default resolver
func checkDNS(domain string, host string, port string) bool {
	var resolver *net.Resolver

	if host != "" && port != "" {
		dnsServer := fmt.Sprintf("%s:%s", host, port)
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: time.Second * 30}
				return d.DialContext(ctx, network, dnsServer)
			},
		}
	} else {
		resolver = net.DefaultResolver
	}

	ips, err := resolver.LookupIP(context.Background(), "ip", domain)
	if err != nil {
		log.Printf("Failed to resolve DNS for %s: %v\n", domain, err)
		return false
	}

	for _, ip := range ips {
		fmt.Println(ip)
	}
	return true
}

func main() {
	r := gin.Default()
	r.LoadHTMLFiles("assets/index.html")
	r.Static("assets/", "./assets")

	r.GET("/kapi", func(c *gin.Context) {
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		portStr := os.Getenv("KUBERNETES_SERVICE_PORT")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, HttpResponse{
				Status: http.StatusInternalServerError,
				Debug:  fmt.Sprintf("Failed to convert port: %v", err),
			})
			return
		}

		status := "closed"
		if checkPort(host, port) {
			status = "open"
		}

		c.JSON(http.StatusOK, HttpResponse{
			Status: http.StatusOK,
			Debug:  fmt.Sprintf("Port %d is %s on %s", port, status, host),
		})
	})

	r.GET("/dns-in", func(c *gin.Context) {
		success := checkDNS("google.com", "", "")
		status := http.StatusOK
		debug := "DNS resolved successfully"
		if !success {
			status = http.StatusInternalServerError
			debug = "DNS resolution failed"
		}

		c.JSON(status, HttpResponse{
			Status: status,
			Debug:  debug,
		})
	})

	r.GET("/dns-out", func(c *gin.Context) {
		success := checkDNS("google.com", "8.8.8.8", "53")
		status := http.StatusOK
		debug := "DNS resolved with external server"
		if !success {
			status = http.StatusInternalServerError
			debug = "DNS resolution failed with external server"
		}

		c.JSON(status, HttpResponse{
			Status: status,
			Debug:  debug,
		})
	})

	r.GET("/ns-check", func(c *gin.Context) {
		c.JSON(http.StatusOK, HttpResponse{
			Status: http.StatusOK,
			Debug:  "ns-check endpoint reached",
		})
	})

	r.GET("/crawler", func(c *gin.Context) {
		url := "https://www.githubstatus.com/"
		timeout := 5

		if envURL, ok := os.LookupEnv("HTTP_URL"); ok {
			url = envURL
		}

		if envTimeout, ok := os.LookupEnv("HTTP_TIMEOUT"); ok {
			if t, err := strconv.Atoi(envTimeout); err == nil {
				timeout = t
			}
		}

		insecureTLS := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		client := &http.Client{
			Timeout:   time.Duration(timeout) * time.Second,
			Transport: insecureTLS,
		}

		resp, err := client.Get(url)
		if err != nil {
			c.JSON(http.StatusInternalServerError, HttpResponse{
				Status: http.StatusInternalServerError,
				Debug:  fmt.Sprintf("Failed to fetch URL: %v", err),
			})
			return
		}
		defer resp.Body.Close()

		c.JSON(resp.StatusCode, HttpResponse{
			Status: resp.StatusCode,
			Os:     runtime.GOOS,
			Debug:  fmt.Sprintf("Fetched URL: %s", url),
		})
		client.CloseIdleConnections()
	})

	r.GET("/", func(c *gin.Context) {
		image := "Calico-Windows-K8s.png"
		if runtime.GOOS == "linux" {
			image = "Calico-Cat-Tigera-Shirt.png"
		}
		c.HTML(http.StatusOK, "index.html", gin.H{
			"image": image,
		})
	})

	r.Run()
}
