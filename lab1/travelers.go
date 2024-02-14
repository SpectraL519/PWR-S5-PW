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
	TravelerId              int16
	NodeState               uint8
	NodeCameraRequestType   uint8
	NodeTravelerRequestType uint8
	NodeResponse            uint8
)

// Constants

const (
	nullTraveler TravelerId = -1

	horizontalBlur string = "--"
	verticalBlur   string = "||"
	noBlur         string = "  "
)

const (
	nodeAvailable   NodeState = iota
	nodeReservedIn  NodeState = iota
	nodeReservedOut NodeState = iota
	nodeOccupied    NodeState = iota
)

const (
	travelerReserveNode NodeTravelerRequestType = iota
	travelerAssignNode  NodeTravelerRequestType = iota
	travelerReleaseNode NodeTravelerRequestType = iota
)

const (
	requestAccepted NodeResponse = iota
	requestDenied   NodeResponse = iota
)

// Global variables

var waitGroup sync.WaitGroup

const sleepDuration time.Duration = 2 * time.Second

// Structures

type TravelerIdRequest struct {
	response chan TravelerId
}

func newTravelerIdRequest() TravelerIdRequest {
	return TravelerIdRequest{response: make(chan TravelerId)}
}

type TravelerIdChannel chan TravelerIdRequest

type TravelerIdManager struct {
	nextId  TravelerId
	maxId   TravelerId
	channel TravelerIdChannel
}

func newTravelerIdManager(maxTravelers TravelerId) *TravelerIdManager {
	return &TravelerIdManager{
		nextId:  0,
		maxId:   maxTravelers,
		channel: make(TravelerIdChannel),
	}
}

func (travelerIdManager *TravelerIdManager) start() {
	defer waitGroup.Done()

	for request := range travelerIdManager.channel {
		if travelerIdManager.nextId < travelerIdManager.maxId {
			request.response <- travelerIdManager.nextId
			travelerIdManager.nextId++
		} else {
			request.response <- nullTraveler
		}
	}
}

type NodeGetStateCameraRequest struct {
	response           chan NodeResponse
	travelerId         chan TravelerId
	horizontalEdgeBlur chan bool
	verticalEdgeBlur   chan bool
}

func newNodeGetStateCameraRequest() NodeGetStateCameraRequest {
	return NodeGetStateCameraRequest{
		response:           make(chan NodeResponse),
		travelerId:         make(chan TravelerId),
		horizontalEdgeBlur: make(chan bool),
		verticalEdgeBlur:   make(chan bool),
	}
}

type NodeCameraRequestChannel chan NodeGetStateCameraRequest

type NodeTravelerRequest struct {
	value      NodeTravelerRequestType
	travelerId TravelerId
	travelerC  Coordinates
	response   chan NodeResponse
}

func newNodeTravelerRequest(
	value NodeTravelerRequestType, id TravelerId, c Coordinates,
) NodeTravelerRequest {
	return NodeTravelerRequest{
		value:      value,
		travelerId: id,
		travelerC:  c,
		response:   make(chan NodeResponse),
	}
}

type NodeTravelerRequestChannel chan NodeTravelerRequest

type Coordinates struct {
	x int
	y int
}

type Node struct {
	c                  Coordinates
	state              NodeState
	travelerId         TravelerId
	horizontalEdgeBlur bool
	verticalEdgeBlur   bool
	cameraChannel      NodeCameraRequestChannel
	travelersChannel   NodeTravelerRequestChannel
}

func newNode(c Coordinates) *Node {
	return &Node{
		c:                  c,
		state:              nodeAvailable,
		travelerId:         nullTraveler,
		horizontalEdgeBlur: false,
		verticalEdgeBlur:   false,
		travelersChannel:   make(NodeTravelerRequestChannel),
		cameraChannel:      make(NodeCameraRequestChannel),
	}
}

func (node *Node) hasTraveler() bool {
	return node.state == nodeOccupied || node.state == nodeReservedOut
}

func (node *Node) start(card *TravelersCard, spawnProb float64, moveProb float64) {
	defer waitGroup.Done()

	for {
		select {
		case request := <-node.cameraChannel:
			node.handleCameraRequest(request)

		case request := <-node.travelersChannel:
			node.handleTravelerRequest(request)

		case <-time.After(sleepDuration):
			if node.state != nodeAvailable {
				continue
			}

			if !(rand.Float64() < spawnProb) {
				continue
			}

			travelerIdRequest := newTravelerIdRequest()
			card.travelerIdManager.channel <- travelerIdRequest

			travelerId := <-travelerIdRequest.response
			if travelerId == nullTraveler {
				continue
			}

			node.state = nodeOccupied
			node.travelerId = travelerId

			newTraveler := Traveler{travelerId, node.c}

			waitGroup.Add(1)
			go newTraveler.start(card, moveProb)
		}
	}
}

func (node *Node) handleCameraRequest(request NodeGetStateCameraRequest) {
	request.response <- requestAccepted
	if node.hasTraveler() {
		request.travelerId <- node.travelerId
	} else {
		request.travelerId <- nullTraveler
	}
	request.horizontalEdgeBlur <- node.horizontalEdgeBlur
	request.verticalEdgeBlur <- node.verticalEdgeBlur
	node.horizontalEdgeBlur = false
	node.verticalEdgeBlur = false
}

func (node *Node) handleTravelerRequest(request NodeTravelerRequest) {
	switch request.value {
	case travelerReserveNode:
		if node.state == nodeAvailable {
			request.response <- requestAccepted

			node.travelerId = request.travelerId
			node.state = nodeReservedIn
		} else {
			request.response <- requestDenied
		}

	case travelerAssignNode:
		if node.state == nodeReservedIn &&
			node.travelerId == request.travelerId {

			request.response <- requestAccepted
			node.state = nodeOccupied

			if node.c.y == request.travelerC.y &&
				node.c.x == request.travelerC.x-1 {
				node.horizontalEdgeBlur = true
			} else if node.c.x == request.travelerC.x &&
				node.c.y == request.travelerC.y-1 {
				node.verticalEdgeBlur = true
			}
		} else {
			request.response <- requestDenied
		}

	case travelerReleaseNode:
		switch node.state {
		case nodeOccupied:
			if node.travelerId == request.travelerId {
				request.response <- requestAccepted
				node.state = nodeReservedOut
			} else {
				request.response <- requestDenied
			}

		case nodeReservedOut:
			if node.travelerId == request.travelerId {
				request.response <- requestAccepted
				node.state = nodeAvailable
				node.travelerId = nullTraveler

				if node.c.y == request.travelerC.y &&
					node.c.x == request.travelerC.x-1 {
					node.horizontalEdgeBlur = true
				} else if node.c.x == request.travelerC.x &&
					node.c.y == request.travelerC.y-1 {
					node.verticalEdgeBlur = true
				}
			} else {
				request.response <- requestDenied
			}

		case nodeReservedIn:
			if node.travelerId == request.travelerId {
				request.response <- requestAccepted
				node.state = nodeAvailable
				node.travelerId = nullTraveler
			} else {
				request.response <- requestDenied
			}

		case nodeAvailable:
			request.response <- requestDenied
		}
	}
}

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

		newC := card.getNewPosition(traveler.c)
		currNode := card.grid[traveler.c.y][traveler.c.x]
		newNode := card.grid[newC.y][newC.x]

		reserveRequest := newNodeTravelerRequest(
			travelerReserveNode, traveler.id, traveler.c)

		newNode.travelersChannel <- reserveRequest
		response := <-reserveRequest.response

		if response == requestAccepted {
			releaseRequest := newNodeTravelerRequest(
				travelerReleaseNode, traveler.id, newC)

			for {
				currNode.travelersChannel <- releaseRequest
				releaseResponse := <-releaseRequest.response
				if releaseResponse == requestAccepted {
					break
				}
			}

			for {
				assignRequest := newNodeTravelerRequest(
					travelerAssignNode, traveler.id, traveler.c)

				newNode.travelersChannel <- assignRequest
				assignResponse := <-assignRequest.response
				if assignResponse == requestAccepted {
					traveler.c = newC
					break
				}
			}

			for {
				currNode.travelersChannel <- releaseRequest
				releaseResponse := <-releaseRequest.response
				if releaseResponse == requestAccepted {
					break
				}
			}
		}
	}
}

type TravelersCard struct {
	height            int
	width             int
	travelerIdManager *TravelerIdManager
	grid              [][]*Node
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
		height:            height,
		width:             width,
		travelerIdManager: travlerIdManager,
		grid:              grid,
	}
}

func (card *TravelersCard) startNodes(spawnProb float64, moveProb float64) {
	defer waitGroup.Done()

	for y := range card.grid {
		for x := range card.grid[y] {
			waitGroup.Add(1)
			go card.grid[y][x].start(card, spawnProb, moveProb)
		}
	}
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
			getNodeStateRequest := newNodeGetStateCameraRequest()

			card.grid[y][x].cameraChannel <- getNodeStateRequest
			<-getNodeStateRequest.response
			travelerId := <-getNodeStateRequest.travelerId
			horizontalEdgeBlur := <-getNodeStateRequest.horizontalEdgeBlur
			verticalEdgeBlurStorage[x] = <-getNodeStateRequest.verticalEdgeBlur

			if travelerId != nullTraveler {
				fmt.Printf("[%02d]", travelerId)
			} else {
				fmt.Printf("[  ]")
			}

			if horizontalEdgeBlur {
				fmt.Printf("%s", horizontalBlur)
			} else {
				fmt.Printf("%s", noBlur)
			}
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
	}
}

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

		fmt.Print("\033[H\033[2J") // clear console
		fmt.Printf("Picture: %d\n", camera.pictureCount)
		camera.card.display()

		time.Sleep(sleepDuration)
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
	go card.startNodes(*travelerSpawnP, *travelerMoveP)

	waitGroup.Add(1)
	go camera.start()

	waitGroup.Wait()
}
