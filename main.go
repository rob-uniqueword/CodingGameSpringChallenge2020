package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

var debug = 1

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
	edges    map[point]bool
	value    float64
	size     int
	children map[*valueCluster]bool
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

func (v *valueCluster) contains(point point) bool {
	for edge := range v.edges {
		if edge == point {
			return true
		}
	}

	for child := range v.children {
		if child.contains(point) {
			return true
		}
	}

	return false
}

func (m *gameMap) initialiseValueClusters() {
	start := time.Now()

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: initialising base clusters", time.Since(start)))

	m.valueGrid = make(map[point]*valueCluster)
	for x := 0; x < m.width; x++ {
		for y := 0; y < m.height; y++ {
			position := point{x, y}
			switch obj := m.grid[position]; obj.(type) {
			case wall:
			default:
				m.valueGrid[position] = &valueCluster{map[point]bool{position: true}, 0, 1, nil, nil}
			}
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: clustering base clusters", time.Since(start)))

	previousClusters := make(map[*valueCluster]bool)
	for _, cluster := range m.valueGrid {
		previousClusters[cluster] = true
	}

	for len(previousClusters) > 1 {
		newClusters := make(map[*valueCluster]bool)
		groupedClusters := make(map[*valueCluster]bool)
		for cluster := range previousClusters {
			if _, grouped := groupedClusters[cluster]; !grouped {
				clusterNeighbourhood := m.getClusterNeighbourhood(cluster, previousClusters, groupedClusters)
				combinedEdges := m.getCombinedEdges(clusterNeighbourhood)

				newCluster := &valueCluster{combinedEdges, 0, 0, clusterNeighbourhood, nil}

				combinedSize := 0
				for neighbour := range clusterNeighbourhood {
					combinedSize += neighbour.size
					groupedClusters[neighbour] = true
					neighbour.parent = newCluster
				}

				newCluster.size = combinedSize
				newClusters[newCluster] = true
			}
		}

		previousClusters = newClusters
	}

	for cluster := range previousClusters {
		m.topCluster = cluster
	}
}

func (m *gameMap) getClusterNeighbourhood(origin *valueCluster, clusters map[*valueCluster]bool, groupedClusters map[*valueCluster]bool) map[*valueCluster]bool {
	adjacentPoints := make(map[point]bool)
	for edge := range origin.edges {
		for point := range m.getNeighbours(edge, 1) {
			adjacentPoints[point] = true
		}
	}

	neighbourhood := map[*valueCluster]bool{origin: true}
	for cluster := range clusters {
		if _, grouped := groupedClusters[cluster]; grouped {
			continue
		}

		isAdjacent := false
		for edge := range cluster.edges {
			for point := range adjacentPoints {
				if edge == point {
					isAdjacent = true
				}
			}
		}

		if isAdjacent {
			neighbourhood[cluster] = true
		}
	}

	return neighbourhood
}

func (m *gameMap) getCombinedEdges(clusters map[*valueCluster]bool) map[point]bool {
	combinedEdges := make(map[point]bool)
	allEdges := make(map[point]int)

	for cluster := range clusters {
		for edge := range cluster.edges {
			allEdges[edge]++
		}
	}

	for edge, count := range allEdges {
		// if it's on the edge of 4 it can't still be an edge
		if count >= 4 {
			continue
		}

		// if any of its neighbours isn't in any cluster it's still an edge
		isRealEdge := false
		for neighbour := range m.getNeighbours(edge, 1) {
			neighbourIsInACluster := false
			for cluster := range clusters {
				if cluster.contains(neighbour) {
					neighbourIsInACluster = true
				}
			}
			if !neighbourIsInACluster {
				isRealEdge = true
			}
		}
		if isRealEdge {
			combinedEdges[edge] = true
		}
	}

	return combinedEdges
}

type gameMap struct {
	currentTurn   int
	width, height int
	myPacs        map[int]pac
	enemyPacs     map[int]pac
	superPellets  map[point]pellet
	grid          map[point]interface{}
	valueGrid     map[point]*valueCluster
	topCluster    *valueCluster
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

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating visible pelets", time.Since(start)))

	visiblePoints := make(map[point]bool)
	for _, pac := range m.myPacs {
		for _, point := range m.getVisiblePoints(pac.position, max(m.height, m.width)) {
			visiblePoints[point] = true
		}
	}

	for point := range visiblePoints {
		switch obj := m.grid[point]; obj.(type) {
		case pellet:
			if obj.(pellet).lastUpdated != m.currentTurn {
				m.add(point, floor{})
			}
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating super pellets", time.Since(start)))

	for position, superPellet := range m.superPellets {
		if superPellet.lastUpdated != m.currentTurn {
			delete(m.superPellets, position)
			m.add(position, floor{})
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

	for position, cluster := range m.valueGrid {
		difference := m.getObjValue(m.grid[position]) - cluster.value
		if difference != 0 {
			cluster.addValue(difference)
		}
	}
}

func (m *gameMap) pathDistance(origin point, target point) int {
	if origin == target {
		return 0
	}

	distance := 0
	viewedPoints := map[point]bool{origin: true}
	lastViewedPoints := map[point]bool{origin: true}

	for len(lastViewedPoints) > 0 {
		distance++
		currentViewedPoints := make(map[point]bool)

		for lastPoint := range lastViewedPoints {
			for _, point := range m.getVisiblePoints(lastPoint, 1) {
				if point == target {
					return distance
				}

				if _, ok := viewedPoints[point]; !ok {
					viewedPoints[point] = true
					currentViewedPoints[point] = true
				}
			}
		}

		lastViewedPoints = currentViewedPoints
	}

	panic("If we got here there's no path and that shouldn't be possible")
}

func (m *gameMap) getNeighbours(origin point, maxDistance int) map[point]int {
	distance := 0
	neighbours := make(map[point]int)
	lastViewedPoints := map[point]bool{origin: true}

	for distance < maxDistance {
		distance++
		currentViewedPoints := make(map[point]bool)

		for lastPoint := range lastViewedPoints {
			for _, point := range m.getVisiblePoints(lastPoint, 1) {
				if _, ok := neighbours[point]; !ok {
					neighbours[point] = distance
					currentViewedPoints[point] = true
				}
			}
		}

		lastViewedPoints = currentViewedPoints
	}

	return neighbours
}

func (m *gameMap) getVisiblePoints(origin point, viewDistance int) []point {
	visiblePoints := make([]point, 0, 4*viewDistance)

	for _, direction := range compass {
		current := point{origin.x, origin.y}
		distanceTravelled := 0

		for distanceTravelled < viewDistance {
			current = current.add(direction, m.width, m.height)

			//fmt.Fprintln(os.Stderr, fmt.Sprintf("current view point = %v", current))

			switch m.grid[current].(type) {
			case wall:
				distanceTravelled = viewDistance
			default:
				visiblePoints = append(visiblePoints, current)
				distanceTravelled++
			}
		}
	}

	return visiblePoints
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

	gameMap := gameMap{0, width, height, make(map[int]pac), make(map[int]pac), make(map[point]pellet), make(map[point]interface{}), nil, nil}

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

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: initialising value clusters", time.Since(start)))

	gameMap.initialiseValueClusters()

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

		gameMap.pathDistance(point{20, 7}, point{21, 9})

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
	var target point

	// temporarily remove this pac so it won't run away from itself
	gameMap.valueGrid[pac.position].addValue(-myPacValue)

	fmt.Fprintln(os.Stderr, fmt.Sprintf("pacID = %v, position = %v", pac.pacID, pac.position))

	for len(bestCluster.children) != 0 {
		bestValue := float64(0)

		for childCluster := range bestCluster.children {

			var nearestPoint point
			distance := 999999

			if childCluster.contains(pac.position) {
				distance = 0
				nearestPoint = pac.position
			} else {
				for edge := range childCluster.edges {
					edgeDistance := gameMap.pathDistance(pac.position, edge)
					if edgeDistance < distance {
						distance = edgeDistance
						nearestPoint = edge
					}
				}
			}

			value := float64(childCluster.value) / float64(childCluster.size)
			if distance == 0 && childCluster.size == 1 {
				value = 0
			} else if distance != 0 {
				value /= float64(distance)
			}

			if pac.pacID == 0 {
				fmt.Fprintln(os.Stderr, fmt.Sprintf("considering: nearestPoint = %v, value = %v, size = %v, distance = %v, calcValue = %v, edges = %v",
					nearestPoint, childCluster.value, childCluster.size, distance, value, childCluster.edges))
			}

			if value >= bestValue {
				bestValue = value
				bestCluster = childCluster
				target = nearestPoint
			}
		}

		fmt.Fprintln(os.Stderr, fmt.Sprintf("best cluster: nearestPoint = %v, value = %v, size = %v, edges = %v",
			target, bestCluster.value, bestCluster.size, bestCluster.edges))
	}

	// put the pac back in the grid
	gameMap.valueGrid[pac.position].addValue(myPacValue)

	return target
}

func manhattanDistance(p1 point, p2 point) int {
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

func max(x int, y int) int {
	if x > y {
		return x
	}
	return y
}

func match(pac1 pac, pac2 pac) bool {
	return pac1.mine == pac2.mine && pac1.pacID == pac2.pacID
}
