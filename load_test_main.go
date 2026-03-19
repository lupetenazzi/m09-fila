package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Telemetry struct {
	DeviceID   string    `json:"device_id"`
	Timestamp  time.Time `json:"timestamp"`
	SensorType string    `json:"sensor_type"`
	ValueType  string    `json:"value_type"`
	Value      float64   `json:"value"`
}

type Metrics struct {
	TotalRequests int64
	SuccessCount  int64
	FailureCount  int64
	ResponseTimes []time.Duration
	mu            sync.Mutex
	StartTime     time.Time
	EndTime       time.Time
}

func (m *Metrics) RecordRequest(duration time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.AddInt64(&m.TotalRequests, 1)
	m.ResponseTimes = append(m.ResponseTimes, duration)

	if success {
		atomic.AddInt64(&m.SuccessCount, 1)
	} else {
		atomic.AddInt64(&m.FailureCount, 1)
	}
}

func (m *Metrics) CalculatePercentile(percentile float64) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.ResponseTimes) == 0 {
		return 0
	}

	times := make([]time.Duration, len(m.ResponseTimes))
	copy(times, m.ResponseTimes)

	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	index := int(float64(len(times)) * percentile / 100)
	if index >= len(times) {
		index = len(times) - 1
	}

	return times[index]
}

func (m *Metrics) GetStats() (min, max, avg time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.ResponseTimes) == 0 {
		return 0, 0, 0
	}

	min = m.ResponseTimes[0]
	max = m.ResponseTimes[0]
	var total time.Duration

	for _, rt := range m.ResponseTimes {
		if rt < min {
			min = rt
		}
		if rt > max {
			max = rt
		}
		total += rt
	}

	avg = total / time.Duration(len(m.ResponseTimes))
	return
}

func sendRequest(baseURL string, deviceID string) (time.Duration, bool, error) {
	telemetry := Telemetry{
		DeviceID:   deviceID,
		Timestamp:  time.Now().UTC(),
		SensorType: "temperature",
		ValueType:  "analog",
		Value:      25.5,
	}

	body, _ := json.Marshal(telemetry)

	start := time.Now()
	resp, err := http.Post(
		baseURL+"/telemetry",
		"application/json",
		bytes.NewBuffer(body),
	)
	duration := time.Since(start)

	if err != nil {
		return duration, false, err
	}
	defer io.ReadAll(resp.Body)
	defer resp.Body.Close()

	return duration, resp.StatusCode == 200, nil
}

func runLoadTest(baseURL string, users, requestsPerUser int, thinkTime time.Duration) *Metrics {
	metrics := &Metrics{
		ResponseTimes: make([]time.Duration, 0),
		StartTime:     time.Now(),
	}

	var wg sync.WaitGroup

	for userID := 0; userID < users; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			for i := 0; i < requestsPerUser; i++ {
				deviceID := fmt.Sprintf("loadtest-user-%d-req-%d", uid, i)
				duration, success, _ := sendRequest(baseURL, deviceID)
				metrics.RecordRequest(duration, success)

				if thinkTime > 0 && i < requestsPerUser-1 {
					time.Sleep(thinkTime)
				}
			}
		}(userID)
	}

	wg.Wait()
	metrics.EndTime = time.Now()
	return metrics
}

func printResults(scenarioName string, metrics *Metrics) {
	totalDuration := metrics.EndTime.Sub(metrics.StartTime)
	successRate := float64(metrics.SuccessCount) / float64(metrics.TotalRequests) * 100
	rps := float64(metrics.TotalRequests) / totalDuration.Seconds()

	min, max, avg := metrics.GetStats()
	p50 := metrics.CalculatePercentile(50)
	p95 := metrics.CalculatePercentile(95)
	p99 := metrics.CalculatePercentile(99)

	fmt.Printf("\n┌─────────────────────────────────────────────────────────┐\n")
	fmt.Printf("│ %s\n", scenarioName)
	fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│ Total Requisições:   %-40d │\n", metrics.TotalRequests)
	fmt.Printf("│ Sucesso:             %-40d (%.2f%%) │\n", metrics.SuccessCount, successRate)
	fmt.Printf("│ Falhas:              %-40d │\n", metrics.FailureCount)
	fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│ Tempo Mín:           %-37dms │\n", min.Milliseconds())
	fmt.Printf("│ Tempo P50:           %-37dms │\n", p50.Milliseconds())
	fmt.Printf("│ Tempo P95:           %-37dms │\n", p95.Milliseconds())
	fmt.Printf("│ Tempo P99:           %-37dms │\n", p99.Milliseconds())
	fmt.Printf("│ Tempo Máx:           %-37dms │\n", max.Milliseconds())
	fmt.Printf("│ Tempo Médio:         %-37.2fms │\n", float64(avg.Milliseconds()))
	fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│ Duração Total:       %-35.2fs │\n", totalDuration.Seconds())
	fmt.Printf("│ Throughput (RPS):    %-39.2f │\n", rps)
	fmt.Printf("└─────────────────────────────────────────────────────────┘\n")
}

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "Backend URL")
	users := flag.Int("users", 10, "Number of concurrent users")
	requests := flag.Int("requests", 50, "Requests per user")
	thinkTime := flag.Duration("think", 100*time.Millisecond, "Think time between requests")
	scenario := flag.String("scenario", "all", "Scenario: light, medium, heavy, all, stress")

	flag.Parse()

	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║          TESTE DE CARGA - TELEMETRIA BACKEND           ║")
	fmt.Println("╚═══════════════════════════════════════════════════════╝")

	switch *scenario {
	case "light":
		fmt.Printf("\nCenário: LIGHT (10 usuários, 100 requisições)\n")
		metrics := runLoadTest(*baseURL, 10, 100, 100*time.Millisecond)
		printResults("CENÁRIO LIGHT", metrics)

	case "medium":
		fmt.Printf("\n Cenário: MEDIUM (50 usuários, 50 requisições)\n")
		metrics := runLoadTest(*baseURL, 50, 50, 50*time.Millisecond)
		printResults("CENÁRIO MEDIUM", metrics)

	case "heavy":
		fmt.Printf("\n Cenário: HEAVY (100 usuários, 50 requisições)\n")
		metrics := runLoadTest(*baseURL, 100, 50, 10*time.Millisecond)
		printResults("CENÁRIO HEAVY", metrics)

	case "stress":
		fmt.Printf("\n Cenário: STRESS (500 usuários, 20 requisições)\n")
		metrics := runLoadTest(*baseURL, 500, 20, 0)
		printResults("CENÁRIO STRESS", metrics)

	case "all":
		fmt.Printf("\n Cenário: LIGHT (10 usuários, 100 requisições)\n")
		metricsLight := runLoadTest(*baseURL, 10, 100, 100*time.Millisecond)
		printResults("CENÁRIO LIGHT", metricsLight)

		time.Sleep(2 * time.Second)

		fmt.Printf("\n Cenário: MEDIUM (50 usuários, 50 requisições)\n")
		metricsMedium := runLoadTest(*baseURL, 50, 50, 50*time.Millisecond)
		printResults("CENÁRIO MEDIUM", metricsMedium)

		time.Sleep(2 * time.Second)

		fmt.Printf("\n Cenário: HEAVY (100 usuários, 50 requisições)\n")
		metricsHeavy := runLoadTest(*baseURL, 100, 50, 10*time.Millisecond)
		printResults("CENÁRIO HEAVY", metricsHeavy)

		time.Sleep(2 * time.Second)

		fmt.Printf("\n Cenário: STRESS (500 usuários, 20 requisições)\n")
		metricsStress := runLoadTest(*baseURL, 500, 20, 0)
		printResults("CENÁRIO STRESS", metricsStress)

	default:
		fmt.Printf("\n Cenário: CUSTOM (%d usuários, %d requisições)\n", *users, *requests)
		metrics := runLoadTest(*baseURL, *users, *requests, *thinkTime)
		printResults("CENÁRIO CUSTOMIZADO", metrics)
	}

	fmt.Println("\n Teste de carga concluído!\n")
}
