package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

//turns
const benchLength = 1000

//varthreadNum := [...]int{3,5,6,7,9,10,11,12,13,14,15}

func BenchmarkGol(b *testing.B) {
	for threads := 1; threads <= 16; threads++ {
		//bitwise to check if power of 2
		//1,2,4,8,16
		os.Stdout = nil // Disable all program output apart from benchmark results
		p := gol.Params{
			Turns:       benchLength,
			Threads:     threads,
			ImageWidth:  512,
			ImageHeight: 512,
		}
		name := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events {

				}
			}
		})
	}
}

//go run golang.org/x/perf/cmd/benchstat -csv results.out | tee results.csv
//go test -run ^$ -timeout 9999m -bench . -benchtime 1x -count 5 | tee results.out
