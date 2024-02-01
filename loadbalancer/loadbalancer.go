package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type LoadBalancerAlgorithm string

type LoadBalancerOption = func(*LoadBalancer)

const (
	RoundRobin      LoadBalancerAlgorithm = "RoundRobin"
	Random          LoadBalancerAlgorithm = "Random"
	LeastConnection LoadBalancerAlgorithm = "LeastConnections"
)

type LoadBalancer struct {
	algorithm       LoadBalancerAlgorithm
	serverPool      *serverPool
	Server          *http.Server
	Port            int
	healthCheckTime int
	maxConnection   int
	mutex           sync.Mutex
}

func WithHealthCheckTime(healthCheckTime int) LoadBalancerOption {
	return func(lb *LoadBalancer) {
		lb.healthCheckTime = healthCheckTime
	}
}

func WithMaxConnection(maxConnection int) LoadBalancerOption {
	return func(lb *LoadBalancer) {
		lb.maxConnection = maxConnection
	}
}

func (l *LoadBalancer) Start() {
	go l.healthCheck()
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", l.Port),
		Handler: http.HandlerFunc(l.loadBalancerHandler),
	}
	l.Server = &server
	log.Print("Starting load balancer server...")
	log.Fatal(server.ListenAndServe())
}

func NewLoadBalancer(serverList string, algorithm LoadBalancerAlgorithm, port int, options ...LoadBalancerOption) (LoadBalancer, error) {
	if len(serverList) == 0 {
		return LoadBalancer{}, errors.New("please provide one or more backends to load balance")
	}

	loadBalancer := LoadBalancer{}
	serverPool := serverPool{
		mux: &sync.RWMutex{},
	}

	tokens := strings.Split(serverList, ",")

	if addressContainsLoopbackWithTargetPort(tokens, port) {
		return LoadBalancer{}, fmt.Errorf("server list must not contain address of the loadbalancer itself :%d", port)
	}

	for _, tok := range tokens {
		serverUrl, err := url.Parse(tok)
		if err != nil {
			log.Fatalf("error while parsing the server url %s %v", tok, err)
			continue
		}
		if !addressIsHttp(serverUrl) {
			log.Printf("backend server must be http(s) %s %v", tok, err)
			continue
		}
		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
			log.Printf("proxy error [%s] %s\n", serverUrl.Host, e.Error())
			retries := getRetryFromContext(request)
			if retries < 3 {
				<-time.After(10 * time.Millisecond)
				ctx := context.WithValue(request.Context(), Retry, retries+1)
				proxy.ServeHTTP(writer, request.WithContext(ctx))
				return
			}
			serverPool.MarkbackendStatus(serverUrl, false)
			attempts := getAttemptsFromContext(request)
			log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
			ctx := context.WithValue(request.Context(), Attempts, attempts+1)
			loadBalancer.loadBalancerHandler(writer, request.WithContext(ctx))
		}

		serverPool.Addbackend(&backend{
			URL:          serverUrl,
			Alive:        true,
			ReverseProxy: proxy,
		})

		log.Printf("Configured server: %s\n", serverUrl)
	}

	lb := LoadBalancer{
		algorithm:       algorithm,
		serverPool:      &serverPool,
		Port:            port,
		healthCheckTime: int(time.Second) * 15,
		maxConnection:   300,
	}

	for _, option := range options {
		option(&lb)
	}

	return lb, nil

}

func (l *LoadBalancer) healthCheck() {
	t := time.NewTicker(time.Duration(l.healthCheckTime))
	for range t.C {
		log.Println("Starting health check...")
		l.serverPool.HealthCheck()
		log.Println("Health check completed")
	}
}

func (l *LoadBalancer) loadBalancerHandler(w http.ResponseWriter, r *http.Request) {
	attempts := getAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}
	l.mutex.Lock()
	peer := l.serverPool.GetNextPeer(l.algorithm)
	l.mutex.Unlock()
	if peer != nil {
		peer.liveConnections.Add(1)
		defer peer.liveConnections.Add(-1)
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

func addressContainsLoopbackWithTargetPort(serverList []string, loadBalancerServerPort int) bool {
	loopbackAddressWithPort := []string{
		fmt.Sprintf("localhost:%d", loadBalancerServerPort),
		fmt.Sprintf("127.0.0.1:%d", loadBalancerServerPort),
		fmt.Sprintf("0.0.0.0:%d", loadBalancerServerPort),
	}
	for _, loopbackAdress := range loopbackAddressWithPort {
		for _, serverAddress := range serverList {
			if strings.Contains(serverAddress, loopbackAdress) {
				return true
			}
		}
	}
	return false
}

func addressIsHttp(url *url.URL) bool {
	return url.Scheme == "http" || url.Scheme == "https"
}
