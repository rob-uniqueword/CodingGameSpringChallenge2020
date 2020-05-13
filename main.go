package main

import (
	"bufio"
	"fmt"
	"os"
)

/**
 * Grab the pellets as fast as you can!
 **/

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1000000), 1000000)

	// width: size of the grid
	// height: top left corner is (x=0, y=0)
	var width, height int
	scanner.Scan()
	fmt.Sscan(scanner.Text(), &width, &height)

	for i := 0; i < height; i++ {
		scanner.Scan()
		//row := scanner.Text() // one line of the grid: space " " is floor, pound "#" is wall
	}
	for {
		var myScore, opponentScore int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &myScore, &opponentScore)
		// visiblePacCount: all your pacs and enemy pacs in sight
		var visiblePacCount int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &visiblePacCount)

		var myPac Pac

		for i := 0; i < visiblePacCount; i++ {
			// pacId: pac number (unique within a team)
			// mine: true if this pac is yours
			// x: position in the grid
			// y: position in the grid
			// typeId: unused in wood leagues
			// speedTurnsLeft: unused in wood leagues
			// abilityCooldown: unused in wood leagues
			var pacId int
			var mine bool
			var _mine int
			var x, y int
			var typeId string
			var speedTurnsLeft, abilityCooldown int
			scanner.Scan()
			fmt.Sscan(scanner.Text(), &pacId, &_mine, &x, &y, &typeId, &speedTurnsLeft, &abilityCooldown)
			mine = _mine != 0

			if mine {
				myPac = Pac{pacId, mine, x, y, typeId, speedTurnsLeft, abilityCooldown}
			}
		}
		// visiblePelletCount: all pellets in sight
		var visiblePelletCount int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &visiblePelletCount)

		var pellets = make([]Pellet, 0, visiblePelletCount)

		for i := 0; i < visiblePelletCount; i++ {
			// value: amount of points this pellet is worth
			var x, y, value int
			scanner.Scan()
			fmt.Sscan(scanner.Text(), &x, &y, &value)

			pellets = append(pellets, Pellet{x, y, value})
		}

		// fmt.Fprintln(os.Stderr, "Debug messages...")
		// fmt.Println("MOVE 0 15 10") // MOVE <pacId> <x> <y>

		var nextPellet = getNextPellet(myPac, pellets)

		fmt.Printf("MOVE %v %v %v\n", myPac.pacID, nextPellet.x, nextPellet.y)
	}
}

func getNextPellet(myPac Pac, pellets []Pellet) Pellet {
	var bestDistance = 9999999
	var bestPellet = Pellet{99999, 99999, 0}

	for _, p := range pellets {
		var distance = Abs(myPac.x-p.x) + Abs(myPac.y-p.y)

		if distance < bestDistance {
			bestDistance = distance
			bestPellet = p
		}
	}

	return bestPellet
}

type Pac struct {
	pacID                           int
	mine                            bool
	x, y                            int
	typeId                          string
	speedTurnsLeft, abilityCooldown int
}

type Pellet struct {
	x     int
	y     int
	value int
}

func Abs(x int) int {
	if x < 0 {
		return -1 * x
	}

	return x
}
