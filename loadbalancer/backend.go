package loadbalancer

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type RetrySettings string

const (
	Attempts RetrySettings = "Attempts"
	Retry    RetrySettings = "Retry"
)

type backend struct {
	URL             *url.URL
	Alive           bool
	mux             sync.RWMutex
	liveConnections atomic.Int64
	ReverseProxy    *httputil.ReverseProxy
}

func (b *backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

func (b *backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.Alive
	b.mux.RUnlock()
	return
}

type serverPool struct {
	backends []*backend
	current  uint64
	mux      *sync.RWMutex
}

func (s *serverPool) Addbackend(backend *backend) {
	s.backends = append(s.backends, backend)
}

func (s *serverPool) NextIndex() int {
	idx := int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
	return idx
}

func (s *serverPool) MarkbackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.backends {
		if b.URL.String() == backendUrl.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (s *serverPool) GetNextPeer(algorithm LoadBalancerAlgorithm) *backend {
	switch algorithm {
	case RoundRobin:
		return s.roundRobin()
	case LeastConnection:
		return s.leastConnections()
	default:
		return s.roundRobin()
	}
}

func (s *serverPool) roundRobin() *backend {
	next := s.NextIndex()
	l := len(s.backends) + next
	for i := next; i < l; i++ {
		idx := i % len(s.backends)
		if s.backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

func (s *serverPool) leastConnections() *backend {
	sort.Slice(s.backends, func(i, j int) bool {
		return s.backends[i].liveConnections.Load() < s.backends[j].liveConnections.Load()
	})
	for _, b := range s.backends {
		if b.IsAlive() {
			return b
		}
	}
	return nil
}

func (s *serverPool) HealthCheck() {
	for _, b := range s.backends {
		status := "up"
		alive := isbackendAlive(b.URL)
		b.SetAlive(alive)
		if !alive {
			status = "down"
		}
		log.Printf("%s [%s]\n", b.URL, status)
	}
}

func getAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 1
}

func getRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func isbackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("Site unreachable, error: ", err)
		return false
	}
	defer conn.Close()
	return true
}
