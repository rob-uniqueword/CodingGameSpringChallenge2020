package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

const myPacValue = float64(-1)
const enemyPacValue = float64(-1)
const pelletValue = float64(1)
const superPelletValue = float64(10)
const panicTime = 48 * time.Millisecond
const clusterNeighbourDistance = 1
const engageGhostsYoungerThan = 2

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
	lastTarget                      point
	lastUpdated                     int
}

func (p pac) fightResult(other pac) int {
	if p.typeID == other.typeID {
		return 0
	}

	if (p.typeID == "ROCK" && other.typeID == "SCISSORS") ||
		(p.typeID == "SCISSORS" && other.typeID == "PAPER") ||
		(p.typeID == "PAPER" && other.typeID == "ROCK") {
		return 1
	}

	return -1
}

type pellet struct {
	value       int
	lastUpdated int
}

type valueCluster struct {
	edges    map[point]bool
	rawValue float64
	value    float64
	size     int
	children map[*valueCluster]bool
	parent   *valueCluster
}

func (v *valueCluster) addValue(value float64) {
	v.rawValue += value
	v.value = (v.rawValue * v.rawValue) / float64(v.size)
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
	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: initialising base clusters", time.Since(m.turnStart)))

	m.valueGrid = make(map[point]*valueCluster)
	for x := 0; x < m.width; x++ {
		for y := 0; y < m.height; y++ {
			position := point{x, y}
			switch obj := m.grid[position]; obj.(type) {
			case wall:
			default:
				m.valueGrid[position] = &valueCluster{map[point]bool{position: true}, 0, 0, 1, nil, nil}
			}
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: clustering base clusters", time.Since(m.turnStart)))

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

				newCluster := &valueCluster{combinedEdges, 0, 0, 0, clusterNeighbourhood, nil}

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
		for point := range m.getNeighbours(edge, clusterNeighbourDistance) {
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
	turnStart     time.Time
	currentTurn   int
	width, height int
	myPacs        map[int]pac
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
	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating visible pellets and enemies", time.Since(m.turnStart)))

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
		case pac:
			if obj.(pac).lastUpdated != m.currentTurn {
				m.add(point, floor{})
			}
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating super pellets", time.Since(m.turnStart)))

	for position, superPellet := range m.superPellets {
		if superPellet.lastUpdated != m.currentTurn {
			delete(m.superPellets, position)
			m.add(position, floor{})
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating my pacs", time.Since(m.turnStart)))

	for _, pac := range m.myPacs {
		if pac.lastUpdated != m.currentTurn {
			delete(m.myPacs, pac.pacID)
			m.add(pac.position, floor{})
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating values", time.Since(m.turnStart)))

	for position, cluster := range m.valueGrid {
		difference := m.getObjValue(m.grid[position]) - cluster.rawValue

		if difference != 0 {
			cluster.addValue(difference)
		}
	}
}

func (m *gameMap) pathDistance(origin point, target point) int {
	result := m.pathDistances(origin, map[point]bool{target: true})
	for _, distance := range result {
		return distance
	}
	panic("If we've got here something is horribly wrong")
}

func (m *gameMap) pathDistances(origin point, targets map[point]bool) map[point]int {
	results := make(map[point]int)

	if _, ok := targets[origin]; ok {
		results[origin] = 0
		if len(targets) == 1 {
			return results
		}
	}

	distance := 0
	viewedPoints := map[point]bool{origin: true}
	lastViewedPoints := map[point]bool{origin: true}

	for len(lastViewedPoints) > 0 {
		distance++
		currentViewedPoints := make(map[point]bool)

		for lastPoint := range lastViewedPoints {
			for _, point := range m.getVisiblePoints(lastPoint, 1) {
				if _, isViewedPoint := viewedPoints[point]; !isViewedPoint {
					if _, isTarget := targets[point]; isTarget {
						results[point] = distance
						if len(results) == len(targets) {
							return results
						}
					}
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
	neighbours := map[point]int{origin: 0}
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

func (m *gameMap) getObjValue(obj interface{}) float64 {
	switch obj.(type) {
	case pac:
		pac := obj.(pac)
		age := float64(m.currentTurn - pac.lastUpdated + 1)
		if pac.mine {
			return myPacValue / age
		}
		return enemyPacValue / age
	case pellet:
		pellet := obj.(pellet)
		age := float64(m.currentTurn - pellet.lastUpdated + 1)
		if pellet.value == 1 {
			return pelletValue / age
		}
		return superPelletValue / age
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
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1000000), 1000000)

	// width: size of the grid
	// height: top left corner is (x=0, y=0)
	var width, height int
	scanner.Scan()
	fmt.Sscan(scanner.Text(), &width, &height)

	fmt.Fprintln(os.Stderr, fmt.Sprintf("making game map"))

	gameMap := gameMap{time.Now(), 0, width, height, make(map[int]pac), make(map[point]pellet), make(map[point]interface{}), nil, nil}

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

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: initialising value clusters", time.Since(gameMap.turnStart)))

	gameMap.initialiseValueClusters()

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: starting game", time.Since(gameMap.turnStart)))

	for {
		var myScore, opponentScore int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &myScore, &opponentScore)

		var visiblePacCount int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &visiblePacCount)

		gameMap.turnStart = time.Now()
		gameMap.currentTurn++

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating pacs", time.Since(gameMap.turnStart)))

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
			newPac := pac{pacID, mine, typeID, speedTurnsLeft, abilityCooldown, position, position, gameMap.currentTurn}

			gameMap.add(position, newPac)
		}

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating pellets", time.Since(gameMap.turnStart)))

		var visiblePelletCount int
		scanner.Scan()
		fmt.Sscan(scanner.Text(), &visiblePelletCount)

		for i := 0; i < visiblePelletCount; i++ {
			var x, y, value int
			scanner.Scan()
			fmt.Sscan(scanner.Text(), &x, &y, &value)

			gameMap.add(point{x, y}, pellet{value, gameMap.currentTurn})
		}

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: updating game map", time.Since(gameMap.turnStart)))

		gameMap.update()

		gameMap.pathDistance(point{20, 7}, point{21, 9})

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: making commands", time.Since(gameMap.turnStart)))

		var commands = make([]string, 0, visiblePacCount)
		for _, pac := range gameMap.myPacs {
			commands = append(commands, chooseAction(pac, gameMap))
		}

		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: printing commands", time.Since(gameMap.turnStart)))

		fmt.Println(strings.Join(commands, "|"))
	}
}

func chooseAction(myPac pac, gameMap gameMap) string {
	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: calculating action for pac %v", time.Since(gameMap.turnStart), myPac.pacID))

	// sprint if possible
	if myPac.abilityCooldown == 0 {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: speeding. speedTurnsLeft = %v, abilityCooldown = %v", time.Since(gameMap.turnStart), myPac.speedTurnsLeft, myPac.abilityCooldown))
		return fmt.Sprintf("SPEED %v", myPac.pacID)
	}

	// check for neighbours
	for point := range gameMap.getNeighbours(myPac.position, 2) {
		switch obj := gameMap.grid[point]; obj.(type) {
		case pac:
			neighbour := obj.(pac)

			fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: squaring up to %v", time.Since(gameMap.turnStart), neighbour))

			// run away from friendly pacs if they have priority
			if neighbour.mine && myPac.pacID != neighbour.pacID {
				escapeRoute := getEscapeRoute(gameMap, myPac.position, neighbour.position)
				fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: pac %v running away from %v, going to %v", time.Since(gameMap.turnStart), myPac.pacID, neighbour.pacID, escapeRoute))
				return fmt.Sprintf("MOVE %v %v %v", myPac.pacID, escapeRoute.x, escapeRoute.y)
			}

			if !neighbour.mine && gameMap.currentTurn-neighbour.lastUpdated < engageGhostsYoungerThan {
				result := myPac.fightResult(neighbour)

				// chase enemies if they look eatable
				if result == 1 && myPac.speedTurnsLeft > neighbour.speedTurnsLeft {
					fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: I am %v, he is %v. Attacking", time.Since(gameMap.turnStart), myPac.typeID, neighbour.typeID))
					return fmt.Sprintf("MOVE %v %v %v", myPac.pacID, neighbour.position.x, neighbour.position.y)
				}

				// run away from enemies we can't eat
				if result < 1 {
					escapeRoute := getEscapeRoute(gameMap, myPac.position, neighbour.position)
					fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: pac %v running away from %v, going to %v", myPac.typeID, time.Since(gameMap.turnStart), neighbour.pacID, escapeRoute))
					return fmt.Sprintf("MOVE %v %v %v", myPac.pacID, escapeRoute.x, escapeRoute.y)
				}
			}
		}
	}

	// run towards the highest value space
	nextTarget := getNextTarget(myPac, gameMap)
	myPac.lastTarget = nextTarget
	return fmt.Sprintf("MOVE %v %v %v", myPac.pacID, nextTarget.x, nextTarget.y)
}

func getEscapeRoute(gameMap gameMap, prey point, predator point) point {
	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: %v escaping from %v", time.Since(gameMap.turnStart), prey, predator))

	movementOptions := gameMap.getNeighbours(prey, 2)

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: options are  %v", time.Since(gameMap.turnStart), movementOptions))

	var predatorDistances map[point]int
	movementOptionMap := make(map[point]bool)
	for point := range movementOptions {
		movementOptionMap[point] = true
	}
	predatorDistances = gameMap.pathDistances(predator, movementOptionMap)

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: predatorDistances are  %v", time.Since(gameMap.turnStart), predatorDistances))

	var bestMovementOption point
	bestDistance := 0
	for point := range movementOptions {
		if distance := predatorDistances[point]; distance >= bestDistance {
			bestMovementOption = point
			bestDistance = distance
		}
	}

	fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: best option is %v with distance %v", time.Since(gameMap.turnStart), bestMovementOption, bestDistance))

	return bestMovementOption
}

func getNextTarget(pac pac, gameMap gameMap) point {
	bestCluster := gameMap.topCluster
	var target point

	// temporarily remove this pac so it won't run away from itself
	gameMap.valueGrid[pac.position].addValue(-myPacValue)
	defer gameMap.valueGrid[pac.position].addValue(myPacValue)

	// fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: position = %v, previousTarget = %v", time.Since(gameMap.turnStart), pac.position, pac.lastTarget))

	for len(bestCluster.children) != 0 {
		// fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: considering %v children", time.Since(gameMap.turnStart), len(bestCluster.children)))

		bestValue := float64(-99999)
		for childCluster := range bestCluster.children {
			// short circuit if we're low on time. It's easier than optimisation
			if turnTime := time.Since(gameMap.turnStart); turnTime > panicTime {
				fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: time to panic. Short circuiting pac %v", turnTime, pac.pacID))
				return pac.lastTarget
			}

			var nearestPoint point
			distance := 999999

			//fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: finding nearest point for %v edges", time.Since(gameMap.turnStart), len(childCluster.edges)))

			if childCluster.contains(pac.position) {
				distance = 0
				nearestPoint = pac.position
			} else {
				for edge, edgeDistance := range gameMap.pathDistances(pac.position, childCluster.edges) {
					if edgeDistance < distance {
						distance = edgeDistance
						nearestPoint = edge
					}
				}
			}

			//fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: calculating value", time.Since(gameMap.turnStart)))

			value := childCluster.value

			if childCluster.size == 1 {
				if distance == 0 || (distance == 1 && pac.speedTurnsLeft != 0) {
					value = 0
				}
			} else if distance != 0 {
				value /= float64(distance)
			}

			//if pac.pacID == 0 {
			// fmt.Fprintln(os.Stderr, fmt.Sprintf("considering: nearestPoint = %v, value = %v, size = %v, distance = %v, calcValue = %v, edges = %v",
			// 	nearestPoint, childCluster.value, childCluster.size, distance, value, childCluster.edges))
			// }

			//fmt.Fprintln(os.Stderr, fmt.Sprintf("%v: comparing result", time.Since(gameMap.turnStart)))

			if value >= bestValue {
				bestValue = value
				bestCluster = childCluster
				target = nearestPoint
			}
		}

		// fmt.Fprintln(os.Stderr, fmt.Sprintf("best cluster: nearestPoint = %v, value = %v, rawValue = %v, size = %v, edges = %v",
		// 	target, bestCluster.value, bestCluster.rawValue, bestCluster.size, bestCluster.edges))
	}

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
