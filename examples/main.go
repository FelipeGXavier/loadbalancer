package main

import (
	"loadbalancer/loadbalancer"
	"log"
	"net/http"
	"time"
)

func stubServers() {
	mux1 := http.NewServeMux()
	mux2 := http.NewServeMux()

	mux1.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second * 5)
		log.Print("Hit :8081")
		w.Write([]byte("Pong"))
	})

	mux2.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second * 5)
		log.Print("Hit :8082")
		w.Write([]byte("Pong"))
	})

	s1 := http.Server{
		Addr:    "localhost:8081",
		Handler: mux1,
	}

	s2 := http.Server{
		Addr:    "localhost:8082",
		Handler: mux2,
	}

	go s1.ListenAndServe()
	go s2.ListenAndServe()
}

func main() {

	stubServers()

	serverList := "http://localhost:8081,http://localhost:8082"

	lb, err := loadbalancer.NewLoadBalancer(serverList, loadbalancer.RoundRobin, 8080)
	if err != nil {
		log.Fatal(err)
	}
	lb.Start()

}
