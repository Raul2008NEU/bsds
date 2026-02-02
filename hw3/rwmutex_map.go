package main

import (
    "fmt"
    "sync"
    "time"
)

type SafeMap struct {
    mu sync.RWMutex
    m  map[int]int
}

func main() {
    sm := SafeMap{m: make(map[int]int)}
    var wg sync.WaitGroup

    start := time.Now()

    for g := 0; g < 50; g++ {
        wg.Add(1)
        go func(g int) {
            defer wg.Done()
            for i := 0; i < 1000; i++ {
                sm.mu.Lock()        // Full lock for writing
                sm.m[g*1000+i] = i
                sm.mu.Unlock()
            }
        }(g)
    }

    wg.Wait()
    elapsed := time.Since(start)

    fmt.Println("Length:", len(sm.m))
    fmt.Println("Time taken:", elapsed)
}
