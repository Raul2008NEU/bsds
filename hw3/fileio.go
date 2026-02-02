package main

import (
    "bufio"
    "fmt"
    "os"
    "time"
)

func unbufferedWrite(filename string, iterations int) time.Duration {
    f, err := os.Create(filename)
    if err != nil {
        panic(err)
    }
    defer f.Close()

    start := time.Now()
    for i := 0; i < iterations; i++ {
        f.Write([]byte(fmt.Sprintf("Line %d\n", i)))
    }
    return time.Since(start)
}

func bufferedWrite(filename string, iterations int) time.Duration {
    f, err := os.Create(filename)
    if err != nil {
        panic(err)
    }
    defer f.Close()
    w := bufio.NewWriter(f)

    start := time.Now()
    for i := 0; i < iterations; i++ {
        w.WriteString(fmt.Sprintf("Line %d\n", i))
    }
    w.Flush()
    return time.Since(start)
}

func main() {
    iterations := 100000

    unbuffered := unbufferedWrite("unbuffered.txt", iterations)
    buffered := bufferedWrite("buffered.txt", iterations)

    fmt.Println("Unbuffered:", unbuffered)
    fmt.Println("Buffered:", buffered)
    fmt.Printf("Buffered is %.2fx faster\n", float64(unbuffered)/float64(buffered))
}
