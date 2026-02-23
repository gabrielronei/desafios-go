package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type result struct {
	statusCode int
	err        error
}

func main() {
	url := flag.String("url", "", "URL do serviço a ser testado")
	requests := flag.Int("requests", 0, "Número total de requests")
	concurrency := flag.Int("concurrency", 1, "Número de chamadas simultâneas")
	flag.Parse()

	if *url == "" || *requests <= 0 || *concurrency <= 0 {
		fmt.Fprintln(os.Stderr, "Uso: --url=<url> --requests=<n> --concurrency=<n>")
		os.Exit(1)
	}

	results := make(chan result, *requests)
	jobs := make(chan struct{}, *requests)

	// Enfileira todos os jobs.
	for i := 0; i < *requests; i++ {
		jobs <- struct{}{}
	}
	close(jobs)

	client := &http.Client{Timeout: 30 * time.Second}
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				resp, err := client.Get(*url)
				if err != nil {
					results <- result{err: err}
					continue
				}
				resp.Body.Close()
				results <- result{statusCode: resp.StatusCode}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	close(results)

	// Contabiliza resultados.
	statusCount := make(map[int]int)
	errors := 0
	for r := range results {
		if r.err != nil {
			errors++
			continue
		}
		statusCount[r.statusCode]++
	}

	// Relatório.
	total := *requests
	fmt.Println("========================================")
	fmt.Println("         RELATÓRIO DE STRESS TEST")
	fmt.Println("========================================")
	fmt.Printf("URL:                    %s\n", *url)
	fmt.Printf("Concorrência:           %d\n", *concurrency)
	fmt.Printf("Tempo total:            %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Total de requests:      %d\n", total)
	fmt.Printf("Requests com HTTP 200:  %d\n", statusCount[200])
	if errors > 0 {
		fmt.Printf("Erros de conexão:       %d\n", errors)
	}
	if len(statusCount) > 1 || (len(statusCount) == 1 && statusCount[200] == 0) {
		fmt.Println("----------------------------------------")
		fmt.Println("Distribuição de status HTTP:")
		for code, count := range statusCount {
			fmt.Printf("  HTTP %d: %d\n", code, count)
		}
	}
	fmt.Println("========================================")
}
