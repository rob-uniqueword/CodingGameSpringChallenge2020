package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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

		var pacs = make([]Pac, 0, visiblePacCount)

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

			pacs = append(pacs, Pac{pacId, mine, x, y, typeId, speedTurnsLeft, abilityCooldown})
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

		var commands = make([]string, 0, visiblePacCount)
		for _, pac := range pacs {
			if pac.mine {
				nextPellet := getNextPellet(pac, pellets, pacs)
				commands = append(commands, fmt.Sprintf("MOVE %v %v %v", pac.pacID, nextPellet.x, nextPellet.y))
			}
		}
		fmt.Println(strings.Join(commands, "|"))
	}
}

func getNextPellet(pac Pac, pellets []Pellet, pacs []Pac) Pellet {
	bestDistance := 9999999
	bestSafeDistance := 9999999
	bestPellet := Pellet{99999, 99999, 0}
	bestSafePellet := Pellet{99999, 99999, 0}
	foundSafePellet := false

	for _, pellet := range pellets {
		worstDanger := 999999
		for _, other := range pacs {
			danger := PacPelletDistance(other, pellet)

			if !Match(other, pac) && danger < worstDanger {
				worstDanger = danger
			}
		}

		var distance = PacPelletDistance(pac, pellet)

		if distance < bestDistance {
			bestDistance = distance
			bestPellet = pellet
		}

		if distance < bestSafeDistance && distance < worstDanger {
			foundSafePellet = true
			bestSafeDistance = distance
			bestSafePellet = pellet
		}
	}

	if foundSafePellet {
		return bestSafePellet
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

func PacPelletDistance(pac Pac, pellet Pellet) int {
	return Distance(pac.x, pac.y, pellet.x, pellet.y)
}

func PacPacDistance(pac1 Pac, pac2 Pac) int {
	return Distance(pac1.x, pac1.y, pac2.x, pac2.y)
}

func Distance(x1 int, y1 int, x2 int, y2 int) int {
	return Abs(x1-x2) + Abs(y1-y2)
}

func Abs(x int) int {
	if x < 0 {
		return -1 * x
	}

	return x
}

func Match(pac1 Pac, pac2 Pac) bool {
	return pac1.mine == pac2.mine && pac1.pacID == pac2.pacID
}
