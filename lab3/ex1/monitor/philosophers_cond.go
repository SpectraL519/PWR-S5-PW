package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Type aliases

type (
	PhilosopherId     uint8
	CutleryId         uint8
	CutleryUsageState int
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
	diningPhilosopherList        []PhilosopherId
	diningPhilosopherListMutex   *sync.Mutex
	philosopherCondList          []*sync.Cond
	cutleryAvailabilityList      []CutleryUsageState
	cutleryAvailabilityListMutex *sync.RWMutex
}

func newMonitor(numPhilosophers PhilosopherId) *Monitor {
	cutleryAvailabilityList := make([]CutleryUsageState, numPhilosophers)
	for i := range cutleryAvailabilityList {
		cutleryAvailabilityList[i] = 2
	}

	philosopherCondList := make([]*sync.Cond, numPhilosophers)
	for i := range philosopherCondList {
		philosopherCondList[i] = sync.NewCond(&sync.Mutex{})
	}

	return &Monitor{
		diningPhilosopherList:        make([]PhilosopherId, 0),
		diningPhilosopherListMutex:   &sync.Mutex{},
		philosopherCondList:          philosopherCondList,
		cutleryAvailabilityList:      cutleryAvailabilityList,
		cutleryAvailabilityListMutex: &sync.RWMutex{},
	}
}

func (monitor *Monitor) acquireCutlery(pId PhilosopherId) {
	monitor.philosopherCondList[pId].L.Lock()
	defer monitor.philosopherCondList[pId].L.Unlock()

	monitor.cutleryAvailabilityListMutex.RLock()
	if monitor.cutleryAvailabilityList[pId] != 2 {
		monitor.cutleryAvailabilityListMutex.RUnlock()
		monitor.philosopherCondList[pId].Wait()
	} else {
		monitor.cutleryAvailabilityListMutex.RUnlock()
	}

	monitor.cutleryAvailabilityListMutex.Lock()
	monitor.cutleryAvailabilityList[left(pId)]--
	monitor.cutleryAvailabilityList[right(pId)]--
	monitor.cutleryAvailabilityListMutex.Unlock()

	monitor.addToDiningList(pId)
}

func (monitor *Monitor) releaseCutlery(pId PhilosopherId) {
	monitor.removeFromDiningList(pId)

	lp := left(pId)
	rp := right(pId)

	monitor.cutleryAvailabilityListMutex.Lock()
	monitor.cutleryAvailabilityList[lp]++
	monitor.cutleryAvailabilityList[rp]++
	monitor.cutleryAvailabilityListMutex.Unlock()

	monitor.cutleryAvailabilityListMutex.RLock()
	if monitor.cutleryAvailabilityList[lp] == 2 {
		monitor.philosopherCondList[lp].Signal()
	}
	if monitor.cutleryAvailabilityList[rp] == 2 {
		monitor.philosopherCondList[rp].Signal()
	}
	monitor.cutleryAvailabilityListMutex.RUnlock()
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
