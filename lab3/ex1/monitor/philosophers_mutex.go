package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Type aliases

type (
	PhilosopherId uint8
	CutleryId     uint8
)

// Global variables

const (
	numPhilosophers    PhilosopherId = 6
	numPhilosophersInt int           = int(numPhilosophers)
)

var (
	mutex     sync.Mutex
	waitGroup sync.WaitGroup
)

// Utility functions

func (pId PhilosopherId) String() string {
	return fmt.Sprintf("(C%d-P%d-C%d)", leftCutlery(pId), pId, rightCutlery(pId))
}

func left(pId PhilosopherId) PhilosopherId {
	return (pId - 1 + numPhilosophers) % numPhilosophers
}

func right(pId PhilosopherId) PhilosopherId {
	return (pId + 1) % numPhilosophers
}

func leftCutlery(pId PhilosopherId) CutleryId {
	return CutleryId(pId)
}

func rightCutlery(pId PhilosopherId) CutleryId {
	return CutleryId(right(pId))
}

// Monitor struct

type Monitor struct {
	diningPhilosopherList      []PhilosopherId
	diningPhilosopherListMutex *sync.Mutex
	cutleryMutexList           []*sync.Mutex
}

func newMonitor(numPhilosophers PhilosopherId) *Monitor {
	cutleryMutexList := make([]*sync.Mutex, numPhilosophers)
	for i := range cutleryMutexList {
		cutleryMutexList[i] = &sync.Mutex{}
	}

	return &Monitor{
		diningPhilosopherList:      make([]PhilosopherId, 0),
		diningPhilosopherListMutex: &sync.Mutex{},
		cutleryMutexList:           cutleryMutexList,
	}
}

func (monitor *Monitor) acquireCutlery(pId PhilosopherId) {
	monitor.cutleryMutexList[leftCutlery(pId)].Lock()
	monitor.cutleryMutexList[rightCutlery(pId)].Lock()

	monitor.addToDiningList(pId)
}

func (monitor *Monitor) releaseCutlery(pId PhilosopherId) {
	monitor.removeFromDiningList(pId)

	monitor.cutleryMutexList[leftCutlery(pId)].Unlock()
	monitor.cutleryMutexList[rightCutlery(pId)].Unlock()
}

func (monitor *Monitor) addToDiningList(pId PhilosopherId) {
	monitor.diningPhilosopherListMutex.Lock()
	defer monitor.diningPhilosopherListMutex.Unlock()

	monitor.diningPhilosopherList = append(monitor.diningPhilosopherList, pId)
	fmt.Println("> ", monitor.diningPhilosopherList)
}

func (monitor *Monitor) removeFromDiningList(pId PhilosopherId) {
	monitor.diningPhilosopherListMutex.Lock()
	defer monitor.diningPhilosopherListMutex.Unlock()

	for i, dpId := range monitor.diningPhilosopherList {
		if dpId != pId {
			continue
		}

		monitor.diningPhilosopherList = append(
			monitor.diningPhilosopherList[:i], monitor.diningPhilosopherList[i+1:]...)
		break
	}

	fmt.Println("> ", monitor.diningPhilosopherList)
}

// Philosopher process

func philosopher(id PhilosopherId, monitor *Monitor) {
	defer waitGroup.Done()

	for {
		think()
		monitor.acquireCutlery(id)
		eat()
		monitor.releaseCutlery(id)
	}
}

func think() {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(1000)+500))
}

func eat() {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(1000)+500))
}

// main

func main() {
	monitor := newMonitor(numPhilosophers)

	for i := 0; i < numPhilosophersInt; i++ {
		waitGroup.Add(1)
		go philosopher(PhilosopherId(i), monitor)
	}

	waitGroup.Wait()
}
