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
	Status int
	Os     string
	Debug  error
}

func checkPort(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	timeout := time.Second * 2

	// Attempt to establish a connection
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false // Port is closed or unreachable
	}
	conn.Close()
	return true // Port is open
}

func checkDNS(domain string, host string, port string) bool {
	var resolver *net.Resolver

	if host != "" && port != "" {
		// Set up a custom resolver with the specified DNS server
		dnsServer := fmt.Sprintf("%s:%s", host, port)
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Use the custom DNS server
				d := net.Dialer{Timeout: time.Second}
				return d.DialContext(ctx, network, dnsServer)
			},
		}
	} else {
		// Use the system's default resolver if no custom DNS server is provided
		resolver = net.DefaultResolver
	}

	// Perform a DNS lookup using the chosen resolver
	ips, err := resolver.LookupIP(context.Background(), "ip", domain)
	if err != nil {
		fmt.Printf("Failed to resolve DNS for %s: %v\n", domain, err)
		return false
	}

	// Print the IP addresses returned from the lookup and return success
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
		// Define the host and port to check
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port, err := strconv.Atoi(os.Getenv("KUBERNETES_SERVICE_PORT"))
		if err != nil {
			fmt.Printf("Failed to convert port: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid port"})
			return
		}

		// Check if the port is open
		if checkPort(host, port) {
			fmt.Printf("Port %d is open on %s\n", port, host)
			c.JSON(http.StatusOK, gin.H{"status": "Port is open"})
		} else {
			fmt.Printf("Port %d is closed on %s\n", port, host)
			c.JSON(http.StatusOK, gin.H{"status": "Port is closed"})
		}
	})

	r.GET("/dns-in", func(c *gin.Context) {
		if checkDNS("google.com", "", "") {
			fmt.Println("dns-in resolved")
			c.JSON(http.StatusOK, gin.H{"status": "DNS resolved"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "DNS resolution failed"})
		}
	})

	r.GET("/dns-out", func(c *gin.Context) {
		if checkDNS("google.com", "8.8.8.8", "53") {
			fmt.Println("dns-out resolved")
			c.JSON(http.StatusOK, gin.H{"status": "DNS resolved with external server"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "DNS resolution failed"})
		}
	})

	r.GET("/ns-check", func(c *gin.Context) {
		fmt.Println("ns-check")
		c.JSON(http.StatusOK, gin.H{"status": "ns-check endpoint reached"})
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
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch URL", "debug": err.Error()})
			return
		}
		defer resp.Body.Close()

		response := HttpResponse{
			Status: resp.StatusCode,
			Os:     runtime.GOOS,
			Debug:  err,
		}

		c.JSON(resp.StatusCode, gin.H{"message": response})
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
