package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type LoadTestConfig struct {
	BaseURL         string
	ConcurrentUsers int
	RequestsPerUser int
	RampUpTime      time.Duration
	TotalDuration   time.Duration
	ThinkTime       time.Duration
}

type LoadTestResult struct {
	TotalRequests     int64
	SuccessRequests   int64
	FailedRequests    int64
	TotalDuration     time.Duration
	MinResponseTime   time.Duration
	MaxResponseTime   time.Duration
	AvgResponseTime   time.Duration
	P95ResponseTime   time.Duration
	P99ResponseTime   time.Duration
	RequestsPerSecond float64
	ResponseTimes     []time.Duration
}

type LoadTestMetrics struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalTime       time.Duration
	ResponseTimes   []time.Duration
	mu              sync.Mutex
}

func (m *LoadTestMetrics) AddResponseTime(duration time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	atomic.AddInt64(&m.TotalRequests, 1)
	m.ResponseTimes = append(m.ResponseTimes, duration)

	if success {
		atomic.AddInt64(&m.SuccessRequests, 1)
	} else {
		atomic.AddInt64(&m.FailedRequests, 1)
	}
}

func sendRequest(baseURL string, deviceID string) (time.Duration, bool, error) {
	telemetry := map[string]interface{}{
		"device_id":   deviceID,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"sensor_type": "temperature",
		"value_type":  "analog",
		"value":       25.5,
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

	success := resp.StatusCode == http.StatusOK
	return duration, success, nil
}

func simulateUser(
	baseURL string,
	userID int,
	requestsPerUser int,
	thinkTime time.Duration,
	metrics *LoadTestMetrics,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for i := 0; i < requestsPerUser; i++ {
		deviceID := fmt.Sprintf("user-%d-device-%d", userID, i)
		duration, success, err := sendRequest(baseURL, deviceID)

		if err != nil {
			metrics.AddResponseTime(duration, false)
		} else {
			metrics.AddResponseTime(duration, success)
		}

		if thinkTime > 0 && i < requestsPerUser-1 {
			time.Sleep(thinkTime)
		}
	}
}

func BenchmarkLoadTest_10Users(b *testing.B) {
	runLoadTest(b, LoadTestConfig{
		BaseURL:         "http://localhost:8080",
		ConcurrentUsers: 10,
		RequestsPerUser: 50,
		ThinkTime:       100 * time.Millisecond,
	})
}

func BenchmarkLoadTest_50Users(b *testing.B) {
	runLoadTest(b, LoadTestConfig{
		BaseURL:         "http://localhost:8080",
		ConcurrentUsers: 50,
		RequestsPerUser: 50,
		ThinkTime:       50 * time.Millisecond,
	})
}

func BenchmarkLoadTest_100Users(b *testing.B) {
	runLoadTest(b, LoadTestConfig{
		BaseURL:         "http://localhost:8080",
		ConcurrentUsers: 100,
		RequestsPerUser: 50,
		ThinkTime:       10 * time.Millisecond,
	})
}

func BenchmarkLoadTest_StressTest(b *testing.B) {
	runLoadTest(b, LoadTestConfig{
		BaseURL:         "http://localhost:8080",
		ConcurrentUsers: 500,
		RequestsPerUser: 20,
		ThinkTime:       0,
	})
}

func runLoadTest(b *testing.B, config LoadTestConfig) {
	metrics := &LoadTestMetrics{
		ResponseTimes: make([]time.Duration, 0),
	}

	var wg sync.WaitGroup
	start := time.Now()

	for userID := 0; userID < config.ConcurrentUsers; userID++ {
		wg.Add(1)
		go simulateUser(
			config.BaseURL,
			userID,
			config.RequestsPerUser,
			config.ThinkTime,
			metrics,
			&wg,
		)
	}

	wg.Wait()
	duration := time.Since(start)

	result := calculateResults(metrics, duration)
	printLoadTestReport(&result, config)

	b.Logf("Total de requisições: %d", result.TotalRequests)
	b.Logf("Requisições bem-sucedidas: %d (%.2f%%)", result.SuccessRequests,
		float64(result.SuccessRequests)/float64(result.TotalRequests)*100)
	b.Logf("Requisições falhadas: %d", result.FailedRequests)
	b.Logf("Tempo médio de resposta: %.2fms", result.AvgResponseTime.Seconds()*1000)
	b.Logf("RPS: %.2f", result.RequestsPerSecond)
}

func calculateResults(metrics *LoadTestMetrics, duration time.Duration) LoadTestResult {
	result := LoadTestResult{
		TotalRequests:   metrics.TotalRequests,
		SuccessRequests: metrics.SuccessRequests,
		FailedRequests:  metrics.FailedRequests,
		TotalDuration:   duration,
		ResponseTimes:   metrics.ResponseTimes,
	}

	if len(metrics.ResponseTimes) > 0 {
		var minTime, maxTime, totalTime time.Duration
		minTime = metrics.ResponseTimes[0]
		maxTime = metrics.ResponseTimes[0]

		for _, t := range metrics.ResponseTimes {
			if t < minTime {
				minTime = t
			}
			if t > maxTime {
				maxTime = t
			}
			totalTime += t
		}

		result.MinResponseTime = minTime
		result.MaxResponseTime = maxTime
		result.AvgResponseTime = totalTime / time.Duration(len(metrics.ResponseTimes))
		result.P95ResponseTime = calculatePercentile(metrics.ResponseTimes, 0.95)
		result.P99ResponseTime = calculatePercentile(metrics.ResponseTimes, 0.99)
	}

	if duration > 0 {
		result.RequestsPerSecond = float64(result.TotalRequests) / duration.Seconds()
	}

	return result
}

func calculatePercentile(times []time.Duration, percentile float64) time.Duration {
	if len(times) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(times))
	copy(sorted, times)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)) * percentile)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func printLoadTestReport(result *LoadTestResult, config LoadTestConfig) {
	fmt.Println("\n" + "================================================================================")
	fmt.Println("TESTE DE CARGA")
	fmt.Println("================================================================================")
	fmt.Printf("\nConfiguração:\n")
	fmt.Printf("  Base URL: %s\n", config.BaseURL)
	fmt.Printf("  Usuários: %d\n", config.ConcurrentUsers)
	fmt.Printf("  Requisições por Usuário: %d\n", config.RequestsPerUser)
	fmt.Printf("\nResultados:\n")
	fmt.Printf("  Total de Requisições: %d\n", result.TotalRequests)
	fmt.Printf("  Bem-sucedidas: %d (%.2f%%)\n",
		result.SuccessRequests,
		float64(result.SuccessRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("  Falhadas: %d\n", result.FailedRequests)
	fmt.Printf("\nTempos de Resposta:\n")
	fmt.Printf("  Mínimo: %.2fms\n", result.MinResponseTime.Seconds()*1000)
	fmt.Printf("  Máximo: %.2fms\n", result.MaxResponseTime.Seconds()*1000)
	fmt.Printf("  Médio: %.2fms\n", result.AvgResponseTime.Seconds()*1000)
	fmt.Printf("  P95: %.2fms\n", result.P95ResponseTime.Seconds()*1000)
	fmt.Printf("  P99: %.2fms\n", result.P99ResponseTime.Seconds()*1000)
	fmt.Printf("\nThroughput:\n")
	fmt.Printf("  Duração Total: %v\n", result.TotalDuration)
	fmt.Printf("  Requisições/s: %.2f\n", result.RequestsPerSecond)
	fmt.Println("================================================================================")
}
