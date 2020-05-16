package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

const myPacValue = -5
const enemyPacValue = -5

var compass = map[rune]point{
	'N': point{0, -1},
	'E': point{1, 0},
	'S': point{0, 1},
	'W': point{-1, 0},
}

type wall struct{}

type floor struct{}

type point struct {
	x, y int
}

func (p point) add(other point, width int, height int) point {
	return point{
		mod(p.x+other.x, width),
		mod(p.y+other.y, height),
	}
}

type pac struct {
	pacID                           int
	mine                            bool
	typeID                          string
	speedTurnsLeft, abilityCooldown int
	position                        point
	lastUpdated                     int
}

type pellet struct {
	value       int
	lastUpdated int
}

type valueCluster struct {
	position point
	value    int
	size     int
	children []*valueCluster
	parent   *valueCluster
}

func (v *valueCluster) addValue(value int) {
	// if v.parent == nil {
	// 	fmt.Fprintln(os.Stderr, fmt.Sprintf("adding %v to %v with parent nil and size %v", value, v.position, v.size))
	// } else {
	// 	fmt.Fprintln(os.Stderr, fmt.Sprintf("adding %v to %v with parent %v and size %v", value, v.position, v.parent.value, v.size))
	// }
	v.value += value
	if v.parent != nil {
		v.parent.addValue(value)
	}
}

func (v *valueCluster) addChildClusters(baseValues map[point]*valueCluster) {
	if v.size == 1 {
		return
	}

	quadrants := make([]map[point]*valueCluster, 0, 4)

	for i := 0; i < 4; i++ {
		quadrants = append(quadrants, make(map[point]*valueCluster))
	}

	centre := v.position

	for position, value := range baseValues {
		var quadrant int

		if position.x <= centre.x && position.y <= centre.y {
			quadrant = 0
		} else if position.x <= centre.x && position.y > centre.y {
			quadrant = 1
		} else if position.x > centre.x && position.y <= centre.y {
			quadrant = 2
		} else {
			quadrant = 3
		}

		quadrants[quadrant][position] = value
	}

	for _, quadrant := range quadrants {
		if len(quadrant) == 0 {
			continue
		}

		var child *valueCluster

		if len(quadrant) == 1 {
			for _, cluster := range quadrant {
				child = cluster
				child.parent = v
			}
		} else {
			child = &valueCluster{getCentre(quadrant), 0, len(quadrant), make([]*valueCluster, 0, 4), v}
		}

		v.children = append(v.children, child)
		child.addChildClusters(quadrant)
	}
}

func makeGameMap(width int, height int) gameMap {
	valueMap := make(map[point]*valueCluster)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			position := point{x, y}
			valueMap[position] = &valueCluster{position, 0, 1, nil, nil}
		}
	}

	topCluster := valueCluster{getCentre(valueMap), 0, len(valueMap), make([]*valueCluster, 0, 4), nil}
	topCluster.addChildClusters(valueMap)

	//fmt.Fprintln(os.Stderr, fmt.Sprintf("child clusters: %v", topCluster.children))
	//for _, cluster := range topCluster.children {
	//	fmt.Fprintln(os.Stderr, fmt.Sprintf("pointer:%v, parent:%v", cluster, *cluster.parent))
	//}

	return gameMap{0, width, height, make(map[int]pac), make(map[int]pac), make(map[point]pellet), make(map[point]interface{}), valueMap, topCluster}
}

type gameMap struct {
	currentTurn   int
	width, height int
	myPacs        map[int]pac
	enemyPacs     map[int]pac
	superPellets  map[point]pellet
	grid          map[point]interface{}
	valueGrid     map[point]*valueCluster
	topCluster    valueCluster
}

func (m *gameMap) add(position point, obj interface{}) {
	switch obj.(type) {
	case pac:
		newPac := obj.(pac)
		pacID := newPac.pacID
		var oldPac pac
		if newPac.mine {
			oldPac = m.myPacs[pacID]
			m.myPacs[pacID] = newPac
		} else {
			oldPac = m.enemyPacs[pacID]
			m.enemyPacs[pacID] = newPac
		}
		m.grid[oldPac.position] = floor{}
		m.grid[position] = newPac
	case pellet:
		newPellet := obj.(pellet)
		if newPellet.value == 10 {
			m.superPellets[position] = newPellet
		}
		m.grid[position] = newPellet
	default:
		m.grid[position] = obj
	}
}

func (m *gameMap) update() {
	start := time.Now()

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating view lines", time.Since(start)))

	for _, pac := range m.myPacs {
		for _, direction := range compass {
			m._updateViewLine(pac.position, direction)
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating super pellets", time.Since(start)))

	for position, superPellet := range m.superPellets {
		if superPellet.lastUpdated != m.currentTurn {
			delete(m.superPellets, position)
			m.grid[position] = floor{}
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating pacs", time.Since(start)))

	for _, pac := range m.myPacs {
		if pac.lastUpdated != m.currentTurn {
			delete(m.myPacs, pac.pacID)
			m.add(pac.position, floor{})
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating values", time.Since(start)))

	for point, obj := range m.grid {
		valueCluster := m.valueGrid[point]
		difference := m.getObjValue(obj) - valueCluster.value
		if difference != 0 {
			valueCluster.addValue(difference)
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updated values", time.Since(start)))
}

func (m *gameMap) _updateViewLine(origin point, direction point) {
	current := point{origin.x, origin.y}

	for {
		current = current.add(direction, m.width, m.height)

		//fmt.Fprintln(os.Stderr, fmt.Sprintf("checking %v, found %t", current, m.grid[current]))

		switch obj := m.grid[current]; obj.(type) {
		case pellet:
			if obj.(pellet).lastUpdated != m.currentTurn {
				m.grid[current] = floor{}
			}
		case wall:
			return
		}
	}
}

func (m *gameMap) getObjValue(obj interface{}) int {
	switch obj.(type) {
	case pac:
		pac := obj.(pac)
		//age := m.currentTurn - pac.lastUpdated + 1
		if pac.mine {
			return myPacValue // / age
		}
		return enemyPacValue // / age
	case pellet:
		pellet := obj.(pellet)
		//age := m.currentTurn - pellet.lastUpdated + 1
		return pellet.value /// age
	default:
		return 0
	}
}

/**
 * Grab the pellets as fast as you can!
 **/

func getCentre(points map[point]*valueCluster) point {
	sumX := 0
	sumY := 0

	for position := range points {
		sumX += position.x
		sumY += position.y
	}

	return point{sumX / len(points), sumY / len(points)}
}

func main() {
	start := time.Now()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1000000), 1000000)

	// width: size of the grid
	// height: top left corner is (x=0, y=0)
	var width, height int
	scanner.Scan()
	fmt.Sscan(scanner.Text(), &width, &height)

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: making game map", time.Since(start)))

	var gameMap = makeGameMap(width, height)

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: populating game map", time.Since(start)))

	for i := 0; i < height; i++ {
		scanner.Scan()
		row := scanner.Text() // one line of the grid: space " " is floor, pound "#" is wall
		for j, char := range row {
			point := point{j, i}

			switch char {
			case ' ':
				gameMap.add(point, pellet{1, 0})
			case '#':
				gameMap.add(point, wall{})
			}
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: starting game", time.Since(start)))

	for {
		gameMap.currentTurn++

		var myScore, opponentScore int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &myScore, &opponentScore)

		var visiblePacCount int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &visiblePacCount)

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating pacs", time.Since(start)))

		for i := 0; i < visiblePacCount; i++ {
			var pacID int
			var _mine int
			var x, y int
			var typeID string
			var speedTurnsLeft, abilityCooldown int
			scanner.Scan()
			fmt.Sscan(scanner.Text(), &pacID, &_mine, &x, &y, &typeID, &speedTurnsLeft, &abilityCooldown)

			mine := _mine != 0
			position := point{x, y}
			newPac := pac{pacID, mine, typeID, speedTurnsLeft, abilityCooldown, position, gameMap.currentTurn}

			gameMap.add(position, newPac)
		}

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating pellets", time.Since(start)))

		var visiblePelletCount int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &visiblePelletCount)

		for i := 0; i < visiblePelletCount; i++ {
			var x, y, value int
			scanner.Scan()
			fmt.Sscan(scanner.Text(), &x, &y, &value)

			gameMap.add(point{x, y}, pellet{value, gameMap.currentTurn})
		}

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating game map", time.Since(start)))

		gameMap.update()

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: making commands", time.Since(start)))

		var commands = make([]string, 0, visiblePacCount)
		for _, pac := range gameMap.myPacs {
			commands = append(commands, chooseAction(pac, gameMap))
		}
		fmt.Println(strings.Join(commands, "|"))
	}
}

func chooseAction(pac pac, gameMap gameMap) string {
	// sprint if possible
	if pac.abilityCooldown <= 1 {
		return fmt.Sprintf("SPEED %v", pac.pacID)
	}

	// if you see an enemy pac and you can eat it, give chase

	// if you see an enemy pac and you can change to eat it, change

	// if you see an enemy pac and you can't eat it or change, run away

	// run towards the highest value space
	nextTarget := getNextTarget(pac, gameMap)
	return fmt.Sprintf("MOVE %v %v %v", pac.pacID, nextTarget.x, nextTarget.y)
}

func getNextTarget(pac pac, gameMap gameMap) point {
	bestCluster := gameMap.topCluster

	// temporarily remove this pac so it won't run away from itself
	gameMap.valueGrid[pac.position].addValue(-myPacValue)

	fmt.Fprintln(os.Stderr, fmt.Sprintf("pacID = %v, position = %v", pac.pacID, pac.position))

	for len(bestCluster.children) != 0 {
		bestValue := float64(0)

		for _, childCluster := range bestCluster.children {
			value := float64(childCluster.value)
			distance := float64(distance(pac.position, childCluster.position))

			if distance == 0 && childCluster.size == 1 {
				value = 0
			} else if distance != 0 {
				value /= distance
			}

			if pac.pacID == 0 {
				fmt.Fprintln(os.Stderr, fmt.Sprintf("considering: position = %v, value = %v, size = %v, distance = %v, calcValue = %v",
					childCluster.position, childCluster.value, childCluster.size, distance, value))
			}

			if value >= bestValue {
				bestValue = value
				bestCluster = *childCluster
			}
		}
		fmt.Fprintln(os.Stderr, fmt.Sprintf("best cluster: position = %v, value = %v, size = %v", bestCluster.position, bestCluster.value, bestCluster.size))
	}

	// put the pac back in the grid
	gameMap.valueGrid[pac.position].addValue(myPacValue)

	return bestCluster.position
}

func distance(p1 point, p2 point) int {
	return abs(p1.x-p2.x) + abs(p1.y-p2.y)
}

func abs(x int) int {
	if x < 0 {
		return -1 * x
	}

	return x
}

func mod(x int, y int) int {
	val := x % y
	for val < 0 {
		val += y
	}
	return val
}

func match(pac1 pac, pac2 pac) bool {
	return pac1.mine == pac2.mine && pac1.pacID == pac2.pacID
}
