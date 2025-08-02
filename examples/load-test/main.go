package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type LoadTestConfig struct {
	OrderServiceURL string
	RequestsPerSec  int
	DurationSec     int
	MaxConcurrent   int
}

type Order struct {
	UserID string  `json:"user_id"`
	Items  []Item  `json:"items"`
	Total  float64 `json:"total"`
}

type Item struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type TestResult struct {
	TotalRequests   int
	SuccessRequests int
	FailedRequests  int
	AvgResponseTime time.Duration
	MaxResponseTime time.Duration
	MinResponseTime time.Duration
}

func main() {
	config := LoadTestConfig{
		OrderServiceURL: getEnvString("ORDER_SERVICE_URL", "http://localhost:8081"),
		RequestsPerSec:  getEnvInt("REQUESTS_PER_SEC", 10),
		DurationSec:     getEnvInt("DURATION_SEC", 60),
		MaxConcurrent:   getEnvInt("MAX_CONCURRENT", 50),
	}

	log.Printf("Starting load test: %d RPS for %d seconds to %s", 
		config.RequestsPerSec, config.DurationSec, config.OrderServiceURL)
	
	result := runLoadTest(config)
	
	printResults(result)
}

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func runLoadTest(config LoadTestConfig) TestResult {
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	result := TestResult{
		MinResponseTime: time.Hour, // Initialize with large value
	}
	
	// Channel to control request rate
	rateLimiter := time.Tick(time.Second / time.Duration(config.RequestsPerSec))
	
	// Channel to control concurrency
	semaphore := make(chan struct{}, config.MaxConcurrent)
	
	// Stop after duration
	stopTime := time.Now().Add(time.Duration(config.DurationSec) * time.Second)
	
	for time.Now().Before(stopTime) {
		<-rateLimiter // Wait for rate limiter
		
		semaphore <- struct{}{} // Acquire semaphore
		wg.Add(1)
		
		go func() {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore
			
			start := time.Now()
			success := makeOrderRequest(config.OrderServiceURL)
			duration := time.Since(start)
			
			mu.Lock()
			result.TotalRequests++
			if success {
				result.SuccessRequests++
			} else {
				result.FailedRequests++
			}
			
			// Update response time stats
			if duration > result.MaxResponseTime {
				result.MaxResponseTime = duration
			}
			if duration < result.MinResponseTime {
				result.MinResponseTime = duration
			}
			
			// Calculate running average (simplified)
			result.AvgResponseTime = (result.AvgResponseTime*time.Duration(result.TotalRequests-1) + duration) / time.Duration(result.TotalRequests)
			mu.Unlock()
		}()
	}
	
	wg.Wait()
	return result
}

func makeOrderRequest(baseURL string) bool {
	// Generate random order
	order := generateRandomOrder()
	
	jsonData, err := json.Marshal(order)
	if err != nil {
		return false
	}
	
	resp, err := http.Post(baseURL+"/orders", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func generateRandomOrder() Order {
	userIDs := []string{"user_001", "user_002", "user_003", "user_004", "user_005"}
	productIDs := []string{"prod_001", "prod_002", "prod_003", "prod_004", "prod_005", "prod_006", "prod_007", "prod_008", "prod_009", "prod_010"}
	
	// Random number of items (1-3)
	itemCount := rand.Intn(3) + 1
	items := make([]Item, itemCount)
	total := 0.0
	
	for i := 0; i < itemCount; i++ {
		price := float64(rand.Intn(10000)+1000) / 100.0 // $10.00 - $109.99
		quantity := rand.Intn(3) + 1                    // 1-3 items
		
		items[i] = Item{
			ProductID: productIDs[rand.Intn(len(productIDs))],
			Quantity:  quantity,
			Price:     price,
		}
		
		total += price * float64(quantity)
	}
	
	return Order{
		UserID: userIDs[rand.Intn(len(userIDs))],
		Items:  items,
		Total:  total,
	}
}

func printResults(result TestResult) {
	fmt.Println("\n=== Load Test Results ===")
	fmt.Printf("Total Requests: %d\n", result.TotalRequests)
	fmt.Printf("Successful: %d (%.2f%%)\n", result.SuccessRequests, float64(result.SuccessRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("Failed: %d (%.2f%%)\n", result.FailedRequests, float64(result.FailedRequests)/float64(result.TotalRequests)*100)
	fmt.Printf("Average Response Time: %v\n", result.AvgResponseTime)
	fmt.Printf("Min Response Time: %v\n", result.MinResponseTime)
	fmt.Printf("Max Response Time: %v\n", result.MaxResponseTime)
	
	// Estimate trace data
	fmt.Println("\n=== Estimated Trace Data ===")
	tracesPerRequest := 3 // order + inventory + payment spans
	totalTraces := result.SuccessRequests * tracesPerRequest
	avgTraceSize := 2048 // ~2KB per trace (estimated)
	totalDataKB := totalTraces * avgTraceSize / 1024
	
	fmt.Printf("Estimated traces generated: %d\n", totalTraces)
	fmt.Printf("Estimated data size: %d KB (%.2f MB)\n", totalDataKB, float64(totalDataKB)/1024)
	fmt.Printf("Data rate: %.2f KB/sec\n", float64(totalDataKB)/60.0)
}