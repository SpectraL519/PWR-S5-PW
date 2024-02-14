package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/akamensky/argparse"
)

// Type aliases

type (
	// General
	TravelerId         int16
	WildTravelerHealth uint8
	DangerZone         int16

	// State enums
	NodeCameraStateE   uint8
	NodeTravelerStateE uint8

	// Request enums
	NodeRequestE       uint8
	NodeResponseE      uint8
	TravelerIdRequestE uint8
)

func (dangerZone DangerZone) active() bool {
	return dangerZone > -1
}

// constants

const ( // general
	bufferSize = 10

	nullTraveler           TravelerId         = -1
	initWildTravelerHealth WildTravelerHealth = 5

	dangerZoneNotActive    DangerZone = -1
	initDangerZoneDuration DangerZone = 3

	dangerZoneMarker string = "##"
	horizontalBlur   string = "--"
	verticalBlur     string = "||"
	noBlur           string = "  "
)

const ( // NodeCameraStateE
	nodeRunning NodeCameraStateE = iota
	nodeBlocekd NodeCameraStateE = iota
)

const ( // NodeTravelerStateE
	nodeAvailable   NodeTravelerStateE = iota
	nodeReservedIn  NodeTravelerStateE = iota
	nodeReservedOut NodeTravelerStateE = iota
	nodeOccupied    NodeTravelerStateE = iota
)

const ( // NodeRequestE
	cameraBlockNode     NodeRequestE = iota
	cameraReleaseNode   NodeRequestE = iota
	travelerReserveNode NodeRequestE = iota
	travelerAssignNode  NodeRequestE = iota
	travelerReleaseNode NodeRequestE = iota
	travelerUnlockNode  NodeRequestE = iota
)

func isCameraRequest(request NodeRequestE) bool {
	return request == cameraBlockNode || request == cameraReleaseNode
}

func isTravelerRequest(request NodeRequestE) bool {
	return !isCameraRequest(request)
}

const ( // NodeResponseE
	requestAccepted   NodeResponseE = iota
	requestDenied     NodeResponseE = iota
	requestSuspended  NodeResponseE = iota
	terminateTraveler NodeResponseE = iota
)

const ( // TravelerIdRequestE
	getId     TravelerIdRequestE = iota
	getWildId TravelerIdRequestE = iota
)

// Global variables

var waitGroup sync.WaitGroup

const sleepDuration time.Duration = 2 * time.Second

// Structures - general

type Coordinates struct {
	x int
	y int
}

// Structures - TravelersCard

type WildTravelerChannel chan Coordinates

type WildTravelerChannelMap map[TravelerId]WildTravelerChannel

type TravelersCard struct {
	height                 int
	width                  int
	travelerIdManager      *TravelerIdManager
	grid                   [][]*Node
	wildTravelerChannelMap WildTravelerChannelMap
}

func newTravelersCard(
	width int, height int, travlerIdManager *TravelerIdManager,
) *TravelersCard {
	grid := make([][]*Node, height)
	for y := range grid {
		grid[y] = make([]*Node, width)
		for x := range grid[y] {
			grid[y][x] = newNode(Coordinates{x, y})
		}
	}

	return &TravelersCard{
		height:                 height,
		width:                  width,
		travelerIdManager:      travlerIdManager,
		grid:                   grid,
		wildTravelerChannelMap: make(WildTravelerChannelMap),
	}
}

func (card *TravelersCard) startNodes(probs *NodeProbs) {
	defer waitGroup.Done()

	for y := range card.grid {
		for x := range card.grid[y] {
			waitGroup.Add(1)
			go card.grid[y][x].start(card, probs)
		}
	}
}

func (card *TravelersCard) isWildTraveler(id TravelerId) bool {
	return id != nullTraveler && id >= card.travelerIdManager.maxId
}

func (card *TravelersCard) getNewPosition(c Coordinates) Coordinates {
	moves := []Coordinates{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	rand.Shuffle(len(moves), func(i, j int) {
		moves[i], moves[j] = moves[j], moves[i]
	})

	for _, move := range moves {
		x, y := c.x+move.x, c.y+move.y
		if x >= 0 && x < card.width && y >= 0 && y < card.height {
			return Coordinates{x, y}
		}
	}

	return c
}

func (card *TravelersCard) display() {
	for y := range card.grid {
		verticalEdgeBlurStorage := make([]bool, card.width)
		for i := range verticalEdgeBlurStorage {
			verticalEdgeBlurStorage[i] = false
		}

		for x := range card.grid[y] {
			request := newNodeCameraRequest(cameraBlockNode)
			card.grid[y][x].requestChannel <- request
			response := <-request.cameraResponse

			for response.response != requestAccepted {
				card.grid[y][x].requestChannel <- request
			}

			if response.dangerZone {
				fmt.Printf("[%s]", dangerZoneMarker)
			} else if response.travelerId != nullTraveler {
				if card.isWildTraveler(response.travelerId) {
					fmt.Printf("[**]")
				} else {
					fmt.Printf("[%02d]", response.travelerId)
				}
			} else {
				fmt.Printf("[  ]")
			}

			if response.horizontalEdgeBlur {
				fmt.Printf("%s", horizontalBlur)
			} else {
				fmt.Printf("%s", noBlur)
			}

			verticalEdgeBlurStorage[x] = response.verticalEdgeBlur
		}

		fmt.Println()
		for i := range verticalEdgeBlurStorage {
			if verticalEdgeBlurStorage[i] {
				fmt.Printf(" %s ", verticalBlur)
			} else {
				fmt.Printf(" %s ", noBlur)
			}
			fmt.Printf("%s", noBlur)
		}
		fmt.Println()

		for x := range card.grid[y] {
			request := newNodeCameraRequest(cameraReleaseNode)
			card.grid[y][x].requestChannel <- request
			response := <-request.cameraResponse

			for response.response != requestAccepted {
				card.grid[y][x].requestChannel <- request
			}
		}
	}
}

// Structures - Camera

type Camera struct {
	pictureCount uint
	card         *TravelersCard
}

func newCamera(card *TravelersCard) *Camera {
	return &Camera{
		pictureCount: 0,
		card:         card,
	}
}

func (camera *Camera) start() {
	defer waitGroup.Done()

	for {
		camera.pictureCount++

		// fmt.Print("\033[H\033[2J") // clear console
		fmt.Printf("Picture: %d\n", camera.pictureCount)
		camera.card.display()

		time.Sleep(sleepDuration)
	}
}

// Structures - Traveler

type Traveler struct {
	id TravelerId
	c  Coordinates
}

func (traveler *Traveler) start(card *TravelersCard, moveProb float64) {
	defer waitGroup.Done()

	for {
		time.Sleep(sleepDuration)

		if rand.Float64() > moveProb {
			continue
		}

		terminate := traveler.move(card)
		if terminate {
			return
		}
	}
}

func (traveler *Traveler) move(card *TravelersCard) bool {
	terminate := false

	newC := card.getNewPosition(traveler.c)
	currNode := card.grid[traveler.c.y][traveler.c.x]
	newNode := card.grid[newC.y][newC.x]

	reserveRequest := newNodeTravelerRequest(travelerReserveNode, traveler.id, traveler.c)
	for {
		newNode.requestChannel <- reserveRequest
		response := <-reserveRequest.travelerResponse
		if response == requestSuspended {
			continue
		}
		if response == requestAccepted {
			break
		}
		return terminate
	}

	releaseRequest := newNodeTravelerRequest(travelerReleaseNode, traveler.id, newC)
	for {
		currNode.requestChannel <- releaseRequest
		response := <-releaseRequest.travelerResponse
		if response == requestAccepted {
			break
		}
	}

	assignRequest := newNodeTravelerRequest(travelerAssignNode, traveler.id, traveler.c)
	for {
		newNode.requestChannel <- assignRequest
		response := <-assignRequest.travelerResponse

		if response == terminateTraveler {
			terminate = true
			break
		} else if response == requestAccepted {
			traveler.c = newC
			break
		}
	}

	for {
		currNode.requestChannel <- releaseRequest
		response := <-releaseRequest.travelerResponse
		if response == requestAccepted {
			break
		}
	}

	return terminate
}

type WildTraveler struct {
	id TravelerId
	c  Coordinates
	hp WildTravelerHealth
}

func newWildTraveler(id TravelerId, c Coordinates) WildTraveler {
	return WildTraveler{
		id: id,
		c:  c,
		hp: initWildTravelerHealth,
	}
}

func (wildTraveler *WildTraveler) alive() bool {
	return wildTraveler.hp > 0
}

func (wildTraveler *WildTraveler) start(card *TravelersCard) {
	defer waitGroup.Done()

	fmt.Fprintf(os.Stderr, "WT %d start! (%d,%d)\n",
		wildTraveler.id, wildTraveler.c.x, wildTraveler.c.y)

	for {
		time.Sleep(sleepDuration)

		select {
		case c := <-card.wildTravelerChannelMap[wildTraveler.id]:
			if c != wildTraveler.c {
				wildTraveler.unlockNode(card, c)
				continue
			}

			fmt.Fprintf(os.Stderr, "WT %d move?\n", wildTraveler.id)
			moved := false
			terminate := false

			possibleMoves := []Coordinates{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
			for _, move := range possibleMoves {
				x, y := wildTraveler.c.x+move.x, wildTraveler.c.y+move.y
				if x < 0 || x >= card.width || y < 0 || y >= card.height {
					fmt.Fprintf(os.Stderr, "WT %d skip (%d,%d)\n", wildTraveler.id, x, y)
					continue
				}

				fmt.Fprintf(os.Stderr, "WT %d move (%d,%d)?\n", wildTraveler.id, x, y)
				moved, terminate = wildTraveler.move(Coordinates{x, y}, card)
				if moved {
					fmt.Fprintf(os.Stderr, "WT %d move [ok] (%d,%d)?\n", wildTraveler.id, x, y)
					break
				}
				if terminate {
					fmt.Fprintf(os.Stderr, "WT %d move [term] (%d,%d)?\n", wildTraveler.id, x, y)
					break
				}
			}

			if !moved {
				wildTraveler.unlockNode(card, wildTraveler.c)
			}
			if terminate {
				delete(card.wildTravelerChannelMap, wildTraveler.id)
				return
			}

		default:
			wildTraveler.hp--
			if !wildTraveler.alive() {
				wildTraveler.terminate(card)
				delete(card.wildTravelerChannelMap, wildTraveler.id)
				return
			}
		}
	}
}

func (wildTraveler *WildTraveler) move(newC Coordinates, card *TravelersCard) (bool, bool) {
	terminate := false

	currNode := card.grid[wildTraveler.c.y][wildTraveler.c.x]
	newNode := card.grid[newC.y][newC.x]

	reserveRequest := newNodeTravelerRequest(travelerReserveNode, wildTraveler.id, wildTraveler.c)
	newNode.requestChannel <- reserveRequest
	response := <-reserveRequest.travelerResponse
	if response != requestAccepted {
		return false, terminate
	}

	fmt.Fprintf(os.Stderr, "WT %d move [reserve] (%d,%d)\n", wildTraveler.id, newC.x, newC.y)

	releaseRequest := newNodeTravelerRequest(travelerReleaseNode, wildTraveler.id, newC)
	for {
		currNode.requestChannel <- releaseRequest
		response = <-releaseRequest.travelerResponse
		if response == requestAccepted {
			break
		}
	}

	fmt.Fprintf(os.Stderr, "WT %d move [release.1] (%d,%d)\n",
		wildTraveler.id, wildTraveler.c.x, wildTraveler.c.y)

	assignRequest := newNodeTravelerRequest(travelerAssignNode, wildTraveler.id, wildTraveler.c)
	for {
		newNode.requestChannel <- assignRequest
		response = <-assignRequest.travelerResponse

		if response == terminateTraveler {
			terminate = true
			break
		}
		if response == requestAccepted {
			wildTraveler.c = newC
			break
		}
	}

	fmt.Fprintf(os.Stderr, "WT %d move [assign] (%d,%d)\n", wildTraveler.id, newC.x, newC.y)

	for {
		currNode.requestChannel <- releaseRequest
		response = <-releaseRequest.travelerResponse
		if response == requestAccepted {
			break
		}
	}

	fmt.Fprintf(os.Stderr, "WT %d move [release.2] (%d,%d)\n",
		wildTraveler.id, wildTraveler.c.x, wildTraveler.c.y)

	return true, terminate
}

func (wildTraveler *WildTraveler) unlockNode(card *TravelersCard, c Coordinates) {
	unlockRequest := newNodeTravelerRequest(
		travelerUnlockNode, wildTraveler.id, wildTraveler.c)
	currNode := card.grid[c.y][c.x]
	for {
		currNode.requestChannel <- unlockRequest
		response := <-unlockRequest.travelerResponse
		if response == requestAccepted {
			break
		}
	}
	fmt.Fprintf(os.Stderr, "WT %d unlock (%d,%d)\n", wildTraveler.id, c.x, c.y)
}

func (wildTraveler *WildTraveler) terminate(card *TravelersCard) {
	currNode := card.grid[wildTraveler.c.y][wildTraveler.c.x]
	releaseRequest := newNodeTravelerRequest(
		travelerReleaseNode, wildTraveler.id, wildTraveler.c)

	finalRelease := false

	for {
		currNode.requestChannel <- releaseRequest
		response := <-releaseRequest.travelerResponse
		if response == requestAccepted {
			if !finalRelease {
				finalRelease = true
				continue
			}
			return
		}
	}
}

// Structures - TravelerIdManager

type TravelerIdChannel chan TravelerId

type TravelerIdRequest struct {
	request  TravelerIdRequestE
	response TravelerIdChannel
}

func newTravelerIdRequest(request TravelerIdRequestE) TravelerIdRequest {
	return TravelerIdRequest{
		request:  request,
		response: make(TravelerIdChannel, bufferSize),
	}
}

type TravelerIdRequestChannel chan TravelerIdRequest

type TravelerIdManager struct {
	nextId     TravelerId
	maxId      TravelerId
	nextWildId TravelerId
	maxWildId  TravelerId
	channel    TravelerIdRequestChannel
}

func newTravelerIdManager(maxTravelers TravelerId) *TravelerIdManager {
	return &TravelerIdManager{
		nextId:     0,
		maxId:      maxTravelers,
		nextWildId: 0,
		maxWildId:  100,
		channel:    make(TravelerIdRequestChannel, bufferSize),
	}
}

func (travelerIdManager *TravelerIdManager) start() {
	defer waitGroup.Done()

	for request := range travelerIdManager.channel {
		switch request.request {
		case getId:
			if travelerIdManager.nextId < travelerIdManager.maxId {
				request.response <- travelerIdManager.nextId
				travelerIdManager.nextId++
			} else {
				request.response <- nullTraveler
			}

		case getWildId:
			request.response <- travelerIdManager.nextWildId + travelerIdManager.maxId
			travelerIdManager.nextWildId++
			travelerIdManager.nextWildId %= travelerIdManager.maxWildId
		}
	}
}

// Structures - Node::Requests

type NodeRequest struct {
	request          NodeRequestE
	travelerData     NodeTravelerRequestData
	cameraResponse   NodeCameraResponseChannel
	travelerResponse NodeTravelerResponseChannel
}

func newNodeCameraRequest(request NodeRequestE) NodeRequest {
	return NodeRequest{
		request:        request,
		cameraResponse: make(NodeCameraResponseChannel, bufferSize),
	}
}

func newNodeTravelerRequest(request NodeRequestE, id TravelerId, c Coordinates) NodeRequest {
	return NodeRequest{
		request: request,
		travelerData: NodeTravelerRequestData{
			id: id,
			c:  c,
		},
		travelerResponse: make(NodeTravelerResponseChannel, bufferSize),
	}
}

type NodeRequestChannel chan NodeRequest

type NodeCameraResponse struct {
	response           NodeResponseE
	travelerId         TravelerId
	dangerZone         bool
	horizontalEdgeBlur bool
	verticalEdgeBlur   bool
}

type NodeCameraResponseChannel chan NodeCameraResponse

type NodeTravelerResponseChannel chan NodeResponseE

type NodeTravelerRequestData struct {
	id TravelerId
	c  Coordinates
}

// Structures - Node

type Node struct {
	c                  Coordinates
	dangerZone         DangerZone
	cameraState        NodeCameraStateE
	travelerState      NodeTravelerStateE
	isWaiting          bool
	travelerId         TravelerId
	horizontalEdgeBlur bool
	verticalEdgeBlur   bool
	requestChannel     NodeRequestChannel
}

func newNode(c Coordinates) *Node {
	return &Node{
		c:                  c,
		dangerZone:         dangerZoneNotActive,
		cameraState:        nodeRunning,
		travelerState:      nodeAvailable,
		isWaiting:          false,
		travelerId:         nullTraveler,
		horizontalEdgeBlur: false,
		verticalEdgeBlur:   false,
		requestChannel:     make(NodeRequestChannel, bufferSize),
	}
}

type NodeProbs struct {
	spawn  float64
	move   float64
	wild   float64
	danger float64
}

func (node *Node) start(
	card *TravelersCard, probs *NodeProbs,
) {
	defer waitGroup.Done()

	for {
		select {
		case request := <-node.requestChannel:
			if isCameraRequest(request.request) {
				node.handleCameraRequest(&request)
			} else if isTravelerRequest(request.request) {
				node.handleTravelerRequest(&request, card)
			}

		case <-time.After(sleepDuration):
			if node.dangerZone.active() {
				node.dangerZone--
			}

			if node.travelerState != nodeAvailable || node.dangerZone.active() {
				continue
			}

			if rand.Float64() < probs.spawn {
				travelerIdRequest := newTravelerIdRequest(getId)
				card.travelerIdManager.channel <- travelerIdRequest

				travelerId := <-travelerIdRequest.response
				if travelerId == nullTraveler {
					continue
				}

				node.travelerState = nodeOccupied
				node.travelerId = travelerId

				newTraveler := Traveler{travelerId, node.c}

				waitGroup.Add(1)
				go newTraveler.start(card, probs.move)
				continue
			}

			if rand.Float64() < probs.wild {
				travelerIdRequest := newTravelerIdRequest(getWildId)
				card.travelerIdManager.channel <- travelerIdRequest

				wildTravelerId := <-travelerIdRequest.response
				card.wildTravelerChannelMap[wildTravelerId] = make(WildTravelerChannel, bufferSize)

				newWildTraveler := newWildTraveler(wildTravelerId, node.c)

				node.travelerState = nodeOccupied
				node.travelerId = wildTravelerId

				waitGroup.Add(1)
				go newWildTraveler.start(card)
				continue
			}

			if rand.Float64() < probs.danger {
				node.dangerZone = initDangerZoneDuration
			}
		}
	}
}

func (node *Node) hasMovement() bool {
	return node.travelerState == nodeReservedIn || node.travelerState == nodeReservedOut
}

func (node *Node) hasTraveler() bool {
	return node.travelerState == nodeOccupied || node.travelerState == nodeReservedOut
}

func (node *Node) reset() {
	node.dangerZone = dangerZoneNotActive
	node.travelerState = nodeAvailable
	node.isWaiting = false
	node.travelerId = nullTraveler
}

func (node *Node) handleCameraRequest(request *NodeRequest) {
	switch request.request {
	case cameraBlockNode:
		node.handleCameraBlockRequest(request)

	case cameraReleaseNode:
		node.handleCameraReleaseRequest(request)

	default:
		request.cameraResponse <- NodeCameraResponse{response: requestDenied}
	}
}

func (node *Node) handleCameraBlockRequest(request *NodeRequest) {
	if node.hasMovement() {
		request.cameraResponse <- NodeCameraResponse{response: requestDenied}
		return
	}

	node.cameraState = nodeBlocekd
	response := NodeCameraResponse{
		response:           requestAccepted,
		dangerZone:         node.dangerZone.active(),
		horizontalEdgeBlur: node.horizontalEdgeBlur,
		verticalEdgeBlur:   node.verticalEdgeBlur,
	}

	if node.dangerZone.active() || !node.hasTraveler() {
		response.travelerId = nullTraveler
	} else {
		response.travelerId = node.travelerId
	}

	node.horizontalEdgeBlur = false
	node.verticalEdgeBlur = false

	request.cameraResponse <- response
}

func (node *Node) handleCameraReleaseRequest(request *NodeRequest) {
	if node.cameraState == nodeRunning {
		request.cameraResponse <- NodeCameraResponse{response: requestDenied}
		return
	}

	request.cameraResponse <- NodeCameraResponse{response: requestAccepted}
	node.cameraState = nodeRunning
}

func (node *Node) handleTravelerRequest(request *NodeRequest, card *TravelersCard) {
	if node.cameraState == nodeBlocekd {
		if node.isWaiting {
			request.travelerResponse <- requestSuspended
		} else {
			request.travelerResponse <- requestDenied
		}
		return
	}

	switch request.request {
	case travelerReserveNode:
		node.handleTravelerReserveRequest(request, card)

	case travelerAssignNode:
		node.handleTravelerAssignRequest(request)

	case travelerReleaseNode:
		node.handleTravelerReleaseRequest(request)

	case travelerUnlockNode:
		if node.isWaiting {
			request.travelerResponse <- requestAccepted
			node.isWaiting = false
		} else {
			request.travelerResponse <- requestDenied
		}

	default:
		request.travelerResponse <- requestDenied
	}
}

func (node *Node) handleTravelerReserveRequest(request *NodeRequest, card *TravelersCard) {
	if node.travelerState == nodeAvailable {
		request.travelerResponse <- requestAccepted
		node.travelerState = nodeReservedIn
		node.travelerId = request.travelerData.id
		return
	}

	if node.travelerState == nodeOccupied && !node.isWaiting &&
		card.isWildTraveler(node.travelerId) && !card.isWildTraveler(request.travelerData.id) {

		fmt.Fprintf(os.Stderr, "T %d move -> (%d,%d)?\n", request.travelerData.id, node.c.x, node.c.y)

		if _, exists := card.wildTravelerChannelMap[node.travelerId]; !exists {
			request.travelerResponse <- requestDenied
			return
		}

		node.isWaiting = true
		fmt.Fprintf(os.Stderr, "WT %d move! (%d,%d)\n", node.travelerId, node.c.x, node.c.y)
		card.wildTravelerChannelMap[node.travelerId] <- node.c
	}

	if node.isWaiting {
		request.travelerResponse <- requestSuspended
	} else {
		request.travelerResponse <- requestDenied
	}
}

func (node *Node) handleTravelerAssignRequest(request *NodeRequest) {
	if node.travelerState != nodeReservedIn ||
		node.travelerId != request.travelerData.id {
		request.travelerResponse <- requestDenied
		return
	}

	if node.dangerZone.active() {
		request.travelerResponse <- terminateTraveler
		node.reset()
		return
	}

	request.travelerResponse <- requestAccepted
	node.travelerState = nodeOccupied

	if node.c.y == request.travelerData.c.y &&
		node.c.x == request.travelerData.c.x-1 {
		node.horizontalEdgeBlur = true
	} else if node.c.x == request.travelerData.c.x &&
		node.c.y == request.travelerData.c.y-1 {
		node.verticalEdgeBlur = true
	}
}

func (node *Node) handleTravelerReleaseRequest(request *NodeRequest) {
	switch node.travelerState {
	case nodeOccupied:
		if node.travelerId == request.travelerData.id {
			request.travelerResponse <- requestAccepted
			node.travelerState = nodeReservedOut
		} else {
			request.travelerResponse <- requestDenied
		}

	case nodeReservedOut:
		if node.travelerId == request.travelerData.id {
			request.travelerResponse <- requestAccepted
			if node.c.y == request.travelerData.c.y &&
				node.c.x == request.travelerData.c.x-1 {
				node.horizontalEdgeBlur = true
			} else if node.c.x == request.travelerData.c.x &&
				node.c.y == request.travelerData.c.y-1 {
				node.verticalEdgeBlur = true
			}
			node.reset()
		} else {
			request.travelerResponse <- requestDenied
		}

	case nodeReservedIn:
		if node.travelerId == request.travelerData.id {
			request.travelerResponse <- requestAccepted
			node.travelerState = nodeAvailable
			node.travelerId = nullTraveler
		} else {
			request.travelerResponse <- requestDenied
		}

	case nodeAvailable:
		request.travelerResponse <- requestDenied
	}
}

// main

func main() {
	parser := argparse.NewParser("travelers", "Multithreaded traveler simulation")

	height := parser.Int("H", "height", &argparse.Options{Default: 4})
	width := parser.Int("W", "width", &argparse.Options{Default: 6})
	maxTravelers := parser.Int("T", "max_travelers", &argparse.Options{Default: 10})

	travelerSpawnP := parser.Float("s", "spawn_prob", &argparse.Options{Default: 0.1})
	travelerMoveP := parser.Float("m", "move_prob", &argparse.Options{Default: 0.5})
	travelerWildP := parser.Float("w", "wild_prob", &argparse.Options{Default: 0.025})
	dangerP := parser.Float("d", "danger_prob", &argparse.Options{Default: 0.025})

	if err := parser.Parse(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "Error: Invalid arguments!")
		fmt.Fprintln(os.Stderr, err.Error())
	}

	const minSize, maxSize int = 1, 10

	if *width < minSize || *width > maxSize {
		fmt.Fprintln(os.Stderr, "Error: Invalid value of width - must be in range [1, 10]")
		os.Exit(1)
	}

	if *height < minSize || *height > maxSize {
		fmt.Fprintln(os.Stderr, "Error: Invalid value of height - must be in range [1, 10]")
		os.Exit(1)
	}

	if *maxTravelers < minSize || *maxTravelers > (*width**height) {
		fmt.Fprintln(
			os.Stderr,
			"Error: Invalid value of max_travelers - must be in range [1, width * height]",
		)
		os.Exit(1)
	}

	travelerIdManager := newTravelerIdManager(TravelerId(*maxTravelers))
	card := newTravelersCard(*width, *height, travelerIdManager)
	camera := newCamera(card)

	waitGroup.Add(1)
	go travelerIdManager.start()

	waitGroup.Add(1)
	go card.startNodes(
		&NodeProbs{
			spawn:  *travelerSpawnP,
			move:   *travelerMoveP,
			wild:   *travelerWildP,
			danger: *dangerP,
		},
	)

	waitGroup.Add(1)
	go camera.start()

	waitGroup.Wait()
}
