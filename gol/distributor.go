package gol

import (
	"fmt"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.
	c.ioCommand <- ioInput
	fp := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	c.ioFilename <- fp
	finalWorld := world(p.ImageHeight, p.ImageWidth)

	turn := 0

	// Populating world with image
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			finalWorld[y][x] = <-c.ioInput
		}
	}

	// TODO: Execute all turns of the Game of Life.
	workerSizeY := p.ImageHeight / p.Threads // 16/4 = 4   64/8 = 8
	//fmt.Println("workerSizeY", workerSizeY)
	wrapper := wrapperCalc(p.ImageHeight, p.ImageWidth)
	workerStartX := 0
	workerEndX := p.ImageWidth

	for turn := 0; turn < p.Turns; turn++ {
		//finalWorld = worldState(finalWorld, p)
		//var worldChSlice []chan [][]uint8
		workerChannelSlice := make([]chan [][]uint8, p.Threads)
		workerChannelSliceTest := make([][]uint8, p.Threads)
		for i := 0; i < p.Threads; i++ {
			workerChannelSlice[i] = make(chan [][]uint8)

			workerStartY := i * workerSizeY
			workerEndY := workerStartY + workerSizeY

			go worker(p, finalWorld, workerStartX, workerEndX, workerStartY, workerEndY, workerChannelSlice[i], wrapper)

		}
		outputWorld := make([][]uint8, 0)
		for i := 0; i < p.Threads; i++ {
			workerChannelSliceTest = <-workerChannelSlice[i]
			//fmt.Println("worker slice len", len(workerChannelSliceTest))
			outputWorld = append(outputWorld, workerChannelSliceTest...)
			fmt.Println(i, len(outputWorld))
		}
		finalWorld = outputWorld
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	//fmt.Println("final final world", finalWorld)
	aliveSlice := calcAlive(p, finalWorld)
	//fmt.Println("count alive", len(aliveSlice))
	c.events <- FinalTurnComplete{turn, aliveSlice}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}

func calcAlive(p Params, world [][]uint8) []util.Cell {
	var cells []util.Cell
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == 255 {
				cells = append(cells, util.Cell{X: j, Y: i})
			}
		}
	}

	return cells

}

func aliveCount(i int, j int, tempWorld [][]uint8, wrapper int) uint8 {
	//check live neighbors
	var count uint8
	count = 0
	for x := i - 1; x <= i+1; x++ {
		for y := j - 1; y <= j+1; y++ {
			if !(x == i && y == j) {
				if tempWorld[(y+wrapper)%len(tempWorld)][(x+wrapper)%len(tempWorld[0])] == 255 {
					//if tempWorld[x&wrapper][y&wrapper] == 255 {
					count++
				}
			}
		}

	}
	return count
}

func worker(p Params, finalWorld [][]uint8, workerStartX int, workerEndX int, workerStartY int, workerEndY int, workerChannel chan<- [][]uint8, wrapper int) {
	workerWorld := makematrix(workerEndY-workerStartY, workerEndX)
	//fmt.Println("FinalWorld b4 Worker", finalWorld)
	//fmt.Println(workerStartX, workerEndX, workerStartY, workerEndY)
	//fmt.Println("Woekr len", len(workerWorld))
	//fmt.Println(workerWorld)
	//remove this
	// make new world if live in old set alive in new
	/*for y := 0; y < workerEndY-workerStartY; y++ {
		for x := 0; x < workerEndX; x++ {
			fmt.Println("workerxVal", x, "WorkeryVal", y)
			workerWorld[y][x] = finalWorld[y][x]
		}
	}*/
	//count := 0
	for y := workerStartY; y < workerEndY; y++ {
		//fmt.Println("y", y)
		for x := workerStartX; x < workerEndX; x++ {
			//fmt.Println("x", x)
			AliveCellsCount := aliveCount(x, y, finalWorld, wrapper)
			//fmt.Println("Alive Cell", AliveCellsCount)

			if finalWorld[y][x] == 255 {
				workerWorld[y-workerStartY][x] = 255
				if AliveCellsCount > 3 {
					//set state of final world here
					workerWorld[y-workerStartY][x] = 0
				} else if AliveCellsCount < 2 {
					workerWorld[y-workerStartY][x] = 0
				}
				//} else if AliveCellsCount == 2 || AliveCellsCount == 3 {
				//	workerWorld[y-workerStartY][x] = 255
				//} else {
				//	workerWorld[y-workerStartY][x] = 255
				//}

			}

			if finalWorld[y][x] == 0 {
				if AliveCellsCount == 3 {
					workerWorld[y-workerStartY][x] = 255
				} else {
					workerWorld[y-workerStartY][x] = 0
				}
			}

			//workerWorld[y][x] = newColor
			//fmt.Println(count)
			//workerWorld[count][x] = newColor

		}
		//count++
		//}
	}
	//fmt.Println("Worker before Channel", workerWorld)
	workerChannel <- workerWorld

}

func makematrix(height int, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func world(ImageHeight int, ImageWidth int) [][]uint8 {
	worldSlice := make([][]uint8, ImageHeight)
	for i := range worldSlice {
		worldSlice[i] = make([]uint8, ImageWidth)
	}
	return worldSlice
}

// Optimization for +16 binary instead as -1 = 15 in binary
func wrapperCalc(ImageHeight int, ImageWidth int) int {
	var wrapper int
	if ImageWidth == 16 && ImageHeight == 16 {
		wrapper = 16

	} else if ImageWidth == 64 && ImageHeight == 64 {
		wrapper = 64
	} else if ImageWidth == 512 && ImageHeight == 512 {
		wrapper = 512

	}
	return wrapper
}

func cellFlipped(c distributorChannels, x int, y int) {
	c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{x, y}}

}
