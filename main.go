package main

import (
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	mode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "demo_mode",
			Help: "Which pathological mode is enabled",
		},
		[]string{"name"},
	)
)

func main() {
	prometheus.MustRegister(mode)

	// HTTP endpoints to trigger patterns
	http.HandleFunc("/leak-goroutines", leakGoroutines)
	http.HandleFunc("/gc-pressure", gcPressure)
	http.HandleFunc("/memory-growth", memoryGrowth)
	http.HandleFunc("/alloc-churn", allocChurn)
	http.HandleFunc("/cpu-blocking", cpuBlocking)

	http.Handle("/metrics", promhttp.Handler())

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 1. Goroutine leak
// curl localhost:8080/leak-goroutines
func leakGoroutines(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("goroutine_leak").Set(1)

	go func() {
		for {
			go func() {
				select {} // never exits
			}()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	w.Write([]byte("started goroutine leak\n"))
}

// 2. GC pause pressure (many short-lived allocations)
// curl localhost:8080/gc-pressure
func gcPressure(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("gc_pressure").Set(1)

	go func() {
		for {
			for i := 0; i < 50_000; i++ {
				_ = make([]byte, 1024) // short-lived garbage
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	w.Write([]byte("started GC pressure\n"))
}

// 3. Heap + RSS growth (long-lived objects)
// curl localhost:8080/memory-growth
func memoryGrowth(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("memory_growth").Set(1)

	var store [][]byte
	go func() {
		for {
			store = append(store, make([]byte, 1_000_000)) // retained
			time.Sleep(500 * time.Millisecond)
		}
	}()

	w.Write([]byte("started memory growth\n"))
}

// 4. Allocation churn without heap growth
// curl localhost:8080/alloc-churn
func allocChurn(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("alloc_churn").Set(1)

	go func() {
		for {
			buf := make([]byte, 1_000)
			_ = buf
		}
	}()

	w.Write([]byte("started allocation churn\n"))
}

// 5. Scheduler / CPU mismatch
// curl localhost:8080/cpu-blocking
func cpuBlocking(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("cpu_blocking").Set(1)

	runtime.GOMAXPROCS(1)
	var mu sync.Mutex

	go func() {
		for {
			mu.Lock()
			time.Sleep(200 * time.Millisecond)
			mu.Unlock()
		}
	}()

	go func() {
		for {
			go func() {
				mu.Lock()
				mu.Unlock()
			}()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	w.Write([]byte("started CPU blocking\n"))
}
