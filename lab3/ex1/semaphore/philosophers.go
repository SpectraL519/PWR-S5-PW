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
	numPhilosophersIdx int           = int(numPhilosophers)
)

var (
	diningPhilosopherList []PhilosopherId
	cutlerySemaphoreList  []Semaphore

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

func addToDiningList(pId PhilosopherId) {
	mutex.Lock()
	defer mutex.Unlock()

	diningPhilosopherList = append(diningPhilosopherList, pId)
	fmt.Println("> ", diningPhilosopherList)
}

func removeFromDiningList(pId PhilosopherId) {
	mutex.Lock()
	defer mutex.Unlock()

	for i, dpId := range diningPhilosopherList {
		if dpId != pId {
			continue
		}

		diningPhilosopherList = append(
			diningPhilosopherList[:i], diningPhilosopherList[i+1:]...)
		break
	}

	fmt.Println("> ", diningPhilosopherList)
}

// Philosopher process

func philosopher(id PhilosopherId) {
	defer waitGroup.Done()

	for {
		think()
		acquireCutlery(id)
		eat()
		releaseCutlery(id)
	}
}

func think() {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(1000)+500))
}

func eat() {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(1000)+200))
}

func acquireCutlery(pId PhilosopherId) {
	cutlerySemaphoreList[leftCutlery(pId)].Acquire()
	cutlerySemaphoreList[rightCutlery(pId)].Acquire()

	addToDiningList(pId)
}

func releaseCutlery(pId PhilosopherId) {
	removeFromDiningList(pId)

	cutlerySemaphoreList[leftCutlery(pId)].Release()
	cutlerySemaphoreList[rightCutlery(pId)].Release()
}

// main

func main() {
	diningPhilosopherList = make([]PhilosopherId, 0)

	cutlerySemaphoreList = make([]Semaphore, 0)
	for i := 0; i < numPhilosophersIdx; i++ {
		cutlerySemaphoreList = append(cutlerySemaphoreList, NewBinarySemaphore())
	}

	for i := 0; i < numPhilosophersIdx; i++ {
		waitGroup.Add(1)
		go philosopher(PhilosopherId(i))
	}

	waitGroup.Wait()
}
