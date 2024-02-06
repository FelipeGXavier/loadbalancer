package loadbalancer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewLoadBalancerEmptyServerList_expect_returnError(t *testing.T) {
	port := 9090
	serverList := ""
	_, err := NewLoadBalancer(serverList, Random, port)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "please provide one or more backends to load balance")
}

func Test_NewLoadBalancerLoopbackAddres_with_samePort_asLb_expect_returnError(t *testing.T) {
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
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lb.serverPool.backends))
}