package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
)

type KVItem struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type KVValue struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Status        string            `json:"status"`
	CacheTag      string            `json:"cache-tag"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
	Author        string            `json:"author"`
	Version       int               `json:"version"`
	IsPublic      bool              `json:"is_public"`
	Tags          []string          `json:"tags"`
	Categories    []string          `json:"categories"`
	Metadata      map[string]string `json:"metadata"`
	Config        map[string]interface{} `json:"config"`
	Statistics    map[string]int    `json:"statistics"`
	RelatedItems  []string          `json:"related_items"`
}

func generateRandomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func generateRandomStringArray(min, max int, length int) []string {
	count := rand.Intn(max-min+1) + min
	result := make([]string, count)
	for i := range result {
		result[i] = generateRandomString(length)
	}
	return result
}

func generateRandomMetadata() map[string]string {
	result := make(map[string]string)
	keys := []string{"source", "region", "language", "priority", "env"}
	values := []string{"api", "web", "cli", "eu", "us", "asia", "en", "es", "fr", "high", "medium", "low", "production", "staging", "development"}
	
	count := rand.Intn(5) + 3 // 3 to 7 items
	for i := 0; i < count; i++ {
		key := keys[rand.Intn(len(keys))]
		value := values[rand.Intn(len(values))]
		result[key] = value
	}
	return result
}

func generateRandomConfig() map[string]interface{} {
	result := make(map[string]interface{})
	
	// Add some random config values
	result["timeout"] = rand.Intn(30) + 5
	result["retry_count"] = rand.Intn(5) + 1
	result["cache_ttl"] = rand.Intn(3600) + 60
	result["max_items"] = rand.Intn(100) + 10
	result["enabled"] = rand.Intn(2) == 1
	result["mode"] = []string{"simple", "advanced", "expert"}[rand.Intn(3)]
	
	// Add nested config
	features := make(map[string]bool)
	featureNames := []string{"search", "filter", "export", "import", "notifications", "analytics"}
	for _, name := range featureNames {
		features[name] = rand.Intn(2) == 1
	}
	result["features"] = features
	
	return result
}

func generateRandomStatistics() map[string]int {
	result := make(map[string]int)
	
	result["views"] = rand.Intn(10000)
	result["likes"] = rand.Intn(1000)
	result["shares"] = rand.Intn(500)
	result["comments"] = rand.Intn(200)
	result["downloads"] = rand.Intn(5000)
	
	return result
}

func main() {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())
	
	// Define the 4 possible cache tag values
	cacheTags := []string{"homepage", "product", "blog", "user"}
	
	// Generate 2,000 items
	items := make([]KVItem, 2000)
	
	for i := 0; i < 2000; i++ {
		// Generate a key
		key := fmt.Sprintf("key_%d_%s", i, generateRandomString(8))
		
		// Evenly distribute the cache tags across the 2,000 items
		// This creates 4 batches of 500 items each with the same cache tag
		cacheTagIndex := i / 500 // Integer division to get batch index (0-3)
		cacheTag := cacheTags[cacheTagIndex]
		
		createdAt := time.Now().Add(-time.Duration(rand.Intn(365)) * 24 * time.Hour).Format(time.RFC3339)
		updatedAt := time.Now().Add(-time.Duration(rand.Intn(30)) * 24 * time.Hour).Format(time.RFC3339)
		
		// Create a large JSON object for the value
		value := KVValue{
			ID:           generateRandomString(16),
			Title:        fmt.Sprintf("Item %d Title", i),
			Description:  fmt.Sprintf("This is a detailed description for item %d with random content: %s", i, generateRandomString(100)),
			Status:       []string{"active", "draft", "archived", "pending"}[rand.Intn(4)],
			CacheTag:     cacheTag,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			Author:       fmt.Sprintf("author_%d@example.com", rand.Intn(10)+1),
			Version:      rand.Intn(10) + 1,
			IsPublic:     rand.Intn(2) == 1,
			Tags:         generateRandomStringArray(2, 8, 8),
			Categories:   generateRandomStringArray(1, 4, 10),
			Metadata:     generateRandomMetadata(),
			Config:       generateRandomConfig(),
			Statistics:   generateRandomStatistics(),
			RelatedItems: generateRandomStringArray(0, 10, 16),
		}
		
		// Add to items
		items[i] = KVItem{
			Key:   key,
			Value: value,
		}
	}
	
	// Write to file
	jsonData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	
	err = os.WriteFile("examples/sample_kv_data.json", jsonData, 0644)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Successfully generated 2,000 items with random cache-tag values (limited to 4 options) and saved to examples/sample_kv_data.json\n")
}