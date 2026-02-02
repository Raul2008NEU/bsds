package main

import (
    "fmt"
    "sync"
    "sync/atomic"
)

func main() {
    // Atomic counter (thread-safe)
    var atomicOps atomic.Uint64
    
    // Regular counter (NOT thread-safe - will show race condition)
    var regularOps int
    
    var wg sync.WaitGroup

    // Spawn 50 goroutines
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 1000; j++ {
                // Atomic increment - always correct
                atomicOps.Add(1)
                
                // Regular increment - race condition!
                regularOps++
            }
        }()
    }

    wg.Wait()

    fmt.Println("Expected value:", 50*1000)
    fmt.Println("Atomic ops:", atomicOps.Load())
    fmt.Println("Regular ops:", regularOps)
}