package statserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/bhmj/goblocks/appstatus"
	"github.com/bhmj/goblocks/log"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	a := assert.New(t)

	// create and run server
	logger, _ := log.New("info", false)
	appStatus := appstatus.New()
	port := getFreeTCPPort()
	server := New(port, logger, appStatus, http.NewServeMux())
	ctx, cancel := context.WithCancel(context.Background())

	go server.Run(ctx)

	reporter, _ := appStatus.GetServiceReporter("dummy service")

	a.True(getAlive(port), "alive must be true")
	a.False(getReady(port), "ready must be false")

	reporter.Ready() // simulate service readiness

	a.True(getReady(port), "ready must be true")

	cancel()
}

func getAlive(port int) bool {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/alive", port), nil)
	started := time.Now()
	for time.Since(started) < 100*time.Millisecond { // wait for the server to start
		resp, err := httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func getReady(port int) bool {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/ready", port), nil)
	resp, err := httpClient.Do(req)
	return err == nil && resp.StatusCode == http.StatusOK
}

func TestHealthcheckServerContextShutdown(t *testing.T) {
	a := assert.New(t)

	// create and run server
	logger, _ := log.New("info", false)
	appStatus := appstatus.New()
	port := getFreeTCPPort()
	server := New(port, logger, appStatus, http.NewServeMux())
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)
	go func() {
		errCh <- server.Run(ctx)
	}()
	time.Sleep(10 * time.Millisecond)

	a.True(getAlive(port)) // server is running

	cancel()
	a.Nil(<-errCh, "no error after shutdown")

	a.False(getAlive(port)) // server is stopped
}

func getFreeTCPPorts(n int) []int {
	var ports []int
	for port := 10000; port < 65535; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			ports = append(ports, port)
			n--
			if n == 0 {
				return ports
			}
		}
	}
	panic(fmt.Sprintf("unable to get free ports (%d)", n))
}

func getFreeTCPPort() int {
	return getFreeTCPPorts(1)[0]
}
