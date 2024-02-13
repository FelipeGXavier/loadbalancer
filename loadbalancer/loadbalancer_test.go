package loadbalancer

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NewLoadBalancerEmptyServerList_expect_returnError(t *testing.T) {
	port := 9090
	serverList := ""
	_, err := NewLoadBalancer(serverList, Random, port)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "please provide one or more backends to load balance")
}

func Test_NewLoadBalancerLoopbackAddress_with_samePortAsLb_expect_returnError(t *testing.T) {
	port := 9090
	serverList := fmt.Sprintf("http://localhost:%d", port)
	_, err := NewLoadBalancer(serverList, Random, port)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), fmt.Sprintf("server list must not contain address of the loadbalancer itself :%d", port))
}

func Test_NewLoadBalancerServerWithNoValidServers_expect_returnError(t *testing.T) {
	port := 9090
	serverPort := 9091
	serverList := fmt.Sprintf("ftp://localhost:%d", serverPort)
	_, err := NewLoadBalancer(serverList, Random, port)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "empty server list, must contain at least one valid backend")
}

func Test_NewLoadBalancerServerWithNotHttpProtocol_expect_beDroppedFromList(t *testing.T) {
	port := 9090
	serverPort := 9091
	serverList := fmt.Sprintf("%s,%s", fmt.Sprintf("ftp://localhost:%d", serverPort), fmt.Sprintf("http://localhost:%d", serverPort))
	lb, err := NewLoadBalancer(serverList, Random, port)
	defer lb.Stop()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lb.serverPool.backends))
}

func Test_RequestWithoutAnyServerOnline_expect_returnError503(t *testing.T) {
	port := 9090
	serverPort := 9091
	serverList := fmt.Sprintf("http://localhost:%d", serverPort)
	lb, err := NewLoadBalancer(serverList, Random, port)

	go func() {
		lb.Start()
	}()

	defer lb.Stop()

	time.Sleep(time.Second * 3)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(lb.serverPool.backends))
	assert.NotNil(t, lb.serverPool)

	r, err := http.Get(fmt.Sprintf("http://localhost:%d", port) + "/ping")

	assert.Nil(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, r.StatusCode)

}

func Test_LoadBalancerRoundRobin(t *testing.T) {
	servers := stubServers()

	for _, testServer := range servers {
		defer testServer.Close()
	}

	port := 9090
	serverPort1 := 9091
	serverPort2 := 9092
	serverPort3 := 9093
	serverList := fmt.Sprintf("%s,%s,%s", fmt.Sprintf("http://localhost:%d", serverPort1), fmt.Sprintf("http://localhost:%d", serverPort2), fmt.Sprintf("http://localhost:%d", serverPort3))
	lb, err := NewLoadBalancer(serverList, RoundRobin, port)

	go func() {
		lb.Start()
	}()

	defer lb.Stop()

	time.Sleep(time.Second * 3)

	assert.Nil(t, err)
	assert.Equal(t, 3, len(lb.serverPool.backends))

	r, err := http.Get(fmt.Sprintf("http://localhost:%d/ping", port))

	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)

	nextRequestServer2 := lb.serverPool.GetNextPeer(lb.algorithm)
	nextRequestServer3 := lb.serverPool.GetNextPeer(lb.algorithm)

	assert.NotNil(t, r)
	assert.NotNil(t, nextRequestServer2)
	assert.NotNil(t, nextRequestServer3)

	expectedHostForSecondReq, _ := url.Parse("http://localhost:9093")
	expectedHostForThirdReq, _ := url.Parse("http://localhost:9091")
	assert.Equal(t, expectedHostForSecondReq.Host, nextRequestServer2.URL.Host)
	assert.Equal(t, expectedHostForThirdReq.Host, nextRequestServer3.URL.Host)
	assert.Equal(t, lb.serverPool.current, uint64(3))

}

func Test_Healthcheck_expectSetBackend_asDown(t *testing.T) {
	port := 9090
	serverPort := 9091
	serverList := fmt.Sprintf("http://localhost:%d", serverPort)
	lb, err := NewLoadBalancer(serverList, Random, port, WithHealthCheckTime(5))

	assert.Nil(t, err)

	go func() {
		lb.Start()
	}()

	time.Sleep(time.Second * 3)

	for _, b := range lb.serverPool.backends {
		assert.True(t, b.IsAlive())
	}

	time.Sleep(time.Second * 5)

	for _, b := range lb.serverPool.backends {
		assert.False(t, b.IsAlive())
	}

}

func stubServers() []*http.Server {
	mux1 := http.NewServeMux()
	mux2 := http.NewServeMux()
	mux3 := http.NewServeMux()

	mux1.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("hit :9091")
		_, err := w.Write([]byte("Pong"))
		if err != nil {
			log.Fatal(err)
		}
	})

	mux2.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("hit :9092")
		_, err := w.Write([]byte("Pong"))
		if err != nil {
			log.Fatal(err)
		}
	})

	mux3.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("hit :9093")
		_, err := w.Write([]byte("Pong"))
		if err != nil {
			log.Fatal(err)
		}
	})

	s1 := http.Server{
		Addr:    "localhost:9091",
		Handler: mux1,
	}

	s2 := http.Server{
		Addr:    "localhost:9092",
		Handler: mux2,
	}

	s3 := http.Server{
		Addr:    "localhost:9093",
		Handler: mux3,
	}

	go s1.ListenAndServe()
	go s2.ListenAndServe()
	go s3.ListenAndServe()

	result := make([]*http.Server, 0)
	result = append(result, &s1)
	result = append(result, &s2)
	result = append(result, &s3)

	return result
}
