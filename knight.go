// Copyright 2012 Sonia Keys
// License MIT: http://www.opensource.org/licenses/MIT

// Adapted from "Enumerating Knight's Tours using an Ant Colony Algorithm"
// by Philip Hingston and Graham Kendal,
// PDF at http://www.cs.nott.ac.uk/~gxk/papers/cec05knights.pdf.

package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const boardSize = 8
const nSquares = boardSize * boardSize
const completeTour = nSquares - 1
const cycles = 27000

// pheromone representation read by ants
var tNet = make([]float64, nSquares*8)

// row, col deltas of legal moves
var drc = [][]int{{1, 2}, {2, 1}, {2, -1}, {1, -2},
	{-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}}

// get square reached by following edge k from square (r, c)
func dest(r, c, k int) (int, int, bool) {
	r += drc[k][0]
	c += drc[k][1]
	return r, c, r >= 0 && r < boardSize && c >= 0 && c < boardSize
}

// struct represents a pheromone amount associated with a move
type rckt struct {
	r, c, k int
	t       float64
}

func main() {
	// waitGroups for ant release clockwork
	var start, reset sync.WaitGroup
	start.Add(1)
	// channel for ants to return tours with pheromone updates
	tch := make(chan []rckt)

	// create an ant for each square
	for r := 0; r < boardSize; r++ {
		for c := 0; c < boardSize; c++ {
			go ant(r, c, &start, &reset, tch)
		}
	}

	// accumulator for new pheromone amounts
	tNew := make([]float64, nSquares*8)

	// map for collecting set of complete tours
	allUnique := make(map[string]int)
	tbuf := make([]byte, 2+completeTour) // for building map key

	// heading
	fmt.Println("Board size:", boardSize)
	fmt.Println("Cycles per repeat:", cycles)
	fmt.Println("          Unique                        Production   Cumm.")
	fmt.Println("        complete     Cumm.      Total   rate         prod.")
	fmt.Println("Repeat     tours    unique   attempts   this repeat  rate")

	// each iteration is a "repeat" as described in the paper
	for repeat := 1; ; repeat++ {
		unique := make(map[string]int) // complete tours this repeat

		// initialize board
		for r := 0; r < boardSize; r++ {
			for c := 0; c < boardSize; c++ {
				for k := 0; k < 8; k++ {
					if _, _, ok := dest(r, c, k); ok {
						tNet[(r*boardSize+c)*8+k] = 1e-6
					}
				}
			}
		}

		// each iteration is a "cycle" as described in the paper
		for i := 0; i < cycles; i++ {
			// evaporate pheromones
			for i := range tNet {
				tNet[i] *= .75
			}

			reset.Add(nSquares) // number of ants to release
			start.Done()        // release them
			reset.Wait()        // wait for them to begin searching
			start.Add(1)        // reset start signal for next cycle

			// gather tours from ants
			for i := 0; i < nSquares; i++ {
				tour := <-tch
				// accumulate complete tours
				if len(tour) == completeTour {
					tbuf[0] = byte(tour[0].r)
					tbuf[1] = byte(tour[0].c)
					for i, m := range tour {
						tbuf[i+2] = byte(m.k)
					}
					key := string(tbuf)
					unique[key] = 1
					allUnique[key] = 1
				}
				// accumulate pheromone amounts from all ants
				for _, move := range tour {
					tNew[(move.r*boardSize+move.c)*8+move.k] += move.t
				}
			}

			// update pheromone amounts on network, reset accumulator
			for i, tn := range tNew {
				tNet[i] += tn
				tNew[i] = 0
			}
		}

		// print statistics:
		//           Unique                        Production   Cumm.
		//         complete     Cumm.       Total   rate         prod.
		// Repeat     tours    unique    attempts   this repeat  rate
		fmt.Printf("%6d %9d %9d %10d   %6.4f       %6.4f\n",
			repeat, len(unique), len(allUnique), repeat*cycles*nSquares,
			float64(len(unique))/float64(cycles*nSquares),
			float64(len(allUnique))/float64(repeat*cycles*nSquares))
	}
}

func printTour(tour []rckt) {
	seq := make([]int, nSquares)
	for i, sq := range tour {
		seq[sq.r*boardSize+sq.c] = i + 1
	}
	last := tour[len(tour)-1]
	r, c, _ := dest(last.r, last.c, last.k)
	seq[r*boardSize+c] = nSquares
	fmt.Println("Move sequence:")
	for r := 0; r < boardSize; r++ {
		for c := 0; c < boardSize; c++ {
			m := seq[r*boardSize+c]
			if m > 0 {
				fmt.Printf(" %3d", seq[r*boardSize+c])
			} else {
				fmt.Print("    ")
			}
		}
		fmt.Println()
	}
}

type square struct {
	r, c int
}

func ant(r, c int, start, reset *sync.WaitGroup, tourCh chan []rckt) {
	rnd := rand.New(rand.NewSource(time.Now().Unix()))
	tabu := make([]square, nSquares)
	moves := make([]rckt, nSquares)
	unexp := make([]rckt, 8)
	tabu[0].r = r
	tabu[0].c = c

	for {
		// cycle initialization
		moves = moves[:0]
		tabu = tabu[:1]
		r := tabu[0].r
		c := tabu[0].c

		// wait for start signal
		start.Wait()
		reset.Done()

		for {
			// choose next move
			unexp = unexp[:0]
			var tSum float64
		findU:
			for k := 0; k < 8; k++ {
				dr, dc, ok := dest(r, c, k)
				if !ok {
					continue
				}
				for _, t := range tabu {
					if t.r == dr && t.c == dc {
						continue findU
					}
				}
				tk := tNet[(r*boardSize+c)*8+k]
				tSum += tk
				// note:  dest r, c stored here
				unexp = append(unexp, rckt{dr, dc, k, tk})
			}
			if len(unexp) == 0 {
				break // no moves
			}
			rn := rnd.Float64() * tSum
			var move rckt
			for _, move = range unexp {
				if rn <= move.t {
					break
				}
				rn -= move.t
			}

			// move to new square
			move.r, r = r, move.r
			move.c, c = c, move.c
			tabu = append(tabu, square{r, c})
			moves = append(moves, move)
		}

		// compute pheromone amount to leave
		for i := range moves {
			moves[i].t = float64(len(moves)-i) / float64(completeTour-i)
		}

		// return tour found for this cycle
		tourCh <- moves
	}
}
