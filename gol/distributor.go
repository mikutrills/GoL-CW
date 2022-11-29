package gol

import (
	"fmt"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

//var completedTurns = 0
var stop = false
var SDLturn int
var turnMutex sync.Mutex

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

//client
//GoL engine
//transfer GoL to server
// distributor divides the work between workers and interacts with other goroutines.

func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
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

			if finalWorld[y][x] == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}

	// TODO: Execute all turns of the Game of Life.

	workerSizeY := p.ImageHeight / p.Threads            // 16/4 = 4   64/8 = 8
	wrapper := wrapperCalc(p.ImageHeight, p.ImageWidth) // For wrapping around the matrix
	workerStartX := 0
	workerEndX := p.ImageWidth
	xtraCheck := p.ImageHeight % p.Threads // !(power of two threads)

	ticker := time.NewTicker(2 * time.Second)
	go sdl_Control(keyPresses, finalWorld, c, p, fp)

	for turn = 0; turn < p.Turns; {
		turnMutex.Lock()
		if stop != true {

			select {

			case <-ticker.C:
				var alive = len(calcAlive(p, finalWorld))
				c.events <- AliveCellsCount{turn, alive}
			default:

				workerChannelSlice := make([]chan [][]uint8, p.Threads)
				workerChannelSliceTest := make([][]uint8, p.Threads)

				for i := 0; i < p.Threads; i++ {
					workerChannelSlice[i] = make(chan [][]uint8)
					workerStartY := i * workerSizeY
					workerEndY := workerStartY + workerSizeY

					if xtraCheck != 0 && i == p.Threads-1 {
						workerEndY = workerStartY + workerSizeY + xtraCheck
						go worker(p, finalWorld, workerStartX, workerEndX, workerStartY, workerEndY, workerChannelSlice[i], wrapper, c, turn)
					} else {
						go worker(p, finalWorld, workerStartX, workerEndX, workerStartY, workerEndY, workerChannelSlice[i], wrapper, c, turn)
					}
				}

				outputWorld := make([][]uint8, 0)

				for i := 0; i < p.Threads; i++ {
					workerChannelSliceTest = <-workerChannelSlice[i]
					outputWorld = append(outputWorld, workerChannelSliceTest...)
					//fmt.Println(i, len(outputWorld))
				}

				finalWorld = outputWorld

				c.events <- TurnComplete{turn}

				turn++

				SDLturn++

				//fmt.Println(SDLturn)
			}
			turnMutex.Unlock()
		} else {
			break
		}

	}

	outputState(c, p, finalWorld, fp, turn)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	aliveSlice := calcAlive(p, finalWorld)
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
				if tempWorld[(y+wrapper)%wrapper][(x+wrapper)%wrapper] == 255 {
					count++
				}
			}
		}

	}
	return count
}

func worker(p Params, finalWorld [][]uint8, workerStartX int, workerEndX int, workerStartY int, workerEndY int, workerChannel chan<- [][]uint8, wrapper int, c distributorChannels, turn int) {
	workerWorld := makematrix(workerEndY-workerStartY, workerEndX)

	for y := workerStartY; y < workerEndY; y++ {

		for x := workerStartX; x < workerEndX; x++ {

			AliveCellsCount := aliveCount(x, y, finalWorld, wrapper)
			workerCalc(finalWorld, workerWorld, x, y, workerStartY, turn, AliveCellsCount, c)

		}
	}
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

func outputState(c distributorChannels, p Params, finalWorld [][]uint8, fp string, turn int) {
	c.ioCommand <- ioOutput
	var filename = fmt.Sprintf("%vx%vx%v", p.ImageHeight, p.ImageWidth, turn)
	c.ioFilename <- filename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- finalWorld[i][j]
		}
	}
	c.events <- ImageOutputComplete{turn, fp}
}

func workerCalc(finalWorld [][]uint8, workerWorld [][]uint8, x int, y int, workerStartY int, turn int, AliveCellsCount uint8, c distributorChannels) {

	if finalWorld[y][x] == 255 {
		workerWorld[y-workerStartY][x] = 255
		if AliveCellsCount > 3 {
			//set state of final world here
			workerWorld[y-workerStartY][x] = 0
			c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
		} else if AliveCellsCount < 2 {
			workerWorld[y-workerStartY][x] = 0
			c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
		}

	}

	if finalWorld[y][x] == 0 {
		if AliveCellsCount == 3 {
			workerWorld[y-workerStartY][x] = 255
			c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: x, Y: y}}
		} else {
			workerWorld[y-workerStartY][x] = 0
		}
	}

}

func sdl_Control(keyPresses <-chan rune, finalWorld [][]uint8, c distributorChannels, p Params, fp string) {
	flag := false
	for {
		select {
		case keypress := <-keyPresses:
			switch keypress {
			case 'p':
				turnMutex.Lock()
				flag = false
				c.events <- StateChange{SDLturn, Paused}
				for {
					if flag == true {
						break
					}
					select {
					case pause := <-keyPresses:
						switch pause {
						case 'p':
							fmt.Println("Continuing")
							c.events <- StateChange{SDLturn, Executing}
							turnMutex.Unlock()
							flag = true
						}
					}
				}
			case 's':
				outputState(c, p, finalWorld, fp, SDLturn)

			case 'q':
				turnMutex.Lock()
				c.events <- StateChange{SDLturn, Quitting}
				turnMutex.Unlock()
				stop = true

			}
		}
	}

}
