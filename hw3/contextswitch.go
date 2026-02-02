package main

import (
    "fmt"
    "runtime"
    "time"
)

func pingPong(iterations int) time.Duration {
    ping := make(chan struct{})
    pong := make(chan struct{})

    // Goroutine 1: receives ping, sends pong
    go func() {
        for i := 0; i < iterations; i++ {
            <-ping
            pong <- struct{}{}
        }
    }()

    // Main goroutine: sends ping, receives pong
    start := time.Now()
    for i := 0; i < iterations; i++ {
        ping <- struct{}{}
        <-pong
    }
    return time.Since(start)
}

func main() {
    iterations := 1000000

    // Test with single OS thread
    runtime.GOMAXPROCS(1)
    singleThread := pingPong(iterations)
    avgSingle := float64(singleThread.Nanoseconds()) / float64(iterations*2)

    // Test with multiple OS threads
    runtime.GOMAXPROCS(runtime.NumCPU())
    multiThread := pingPong(iterations)
    avgMulti := float64(multiThread.Nanoseconds()) / float64(iterations*2)

    fmt.Printf("Single thread: %v (avg %.2f ns/switch)\n", singleThread, avgSingle)
    fmt.Printf("Multi thread:  %v (avg %.2f ns/switch)\n", multiThread, avgMulti)
}