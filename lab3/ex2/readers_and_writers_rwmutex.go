package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Type aliases

type (
	ReaderId uint8
	WriterId int16

	ReaderList []ReaderId
)

// Global variables

const (
	numReaders    ReaderId = 3
	numReadersInt int      = int(numReaders)

	nullWriter    WriterId = -1
	numWriters    WriterId = 2
	numWritersInt int      = int(numWriters)
)

var waitGroup sync.WaitGroup

// Utility functions

func (rId ReaderId) String() string {
	return fmt.Sprintf("R%d", rId)
}

func (wId WriterId) String() string {
	return fmt.Sprintf("[W%d]", wId)
}

func randomSleep() {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(1000)+500))
}

// Monitor struct

type Monitor struct {
	readers ReaderList
	writer  WriterId
	mutex   *sync.Mutex
	rwMutex *sync.RWMutex

	writersMutex *sync.Mutex
}

func newMonitor() *Monitor {
	return &Monitor{
		readers: make(ReaderList, 0),
		writer:  nullWriter,
		mutex:   &sync.Mutex{},
		rwMutex: &sync.RWMutex{},
	}
}

func (monitor *Monitor) displayAttendanceList() {
	if monitor.isWriterPresent() {
		fmt.Println(">", monitor.writer)
	} else {
		fmt.Println(">", monitor.readers)
	}
}

func (monitor *Monitor) isWriterPresent() bool {
	return monitor.writer != nullWriter
}

func (monitor *Monitor) areReadersPresent() bool {
	return len(monitor.readers) > 0
}

func (monitor *Monitor) addReader(rId ReaderId) {
	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()

	monitor.readers = append(monitor.readers, rId)
	monitor.displayAttendanceList()
}

func (monitor *Monitor) removeReader(rId ReaderId) {
	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()

	for i, prId := range monitor.readers {
		if prId != rId {
			continue
		}

		monitor.readers = append(monitor.readers[:i], monitor.readers[i+1:]...)
		break
	}

	monitor.displayAttendanceList()
}

func (monitor *Monitor) addWriter(wId WriterId) {
	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()

	monitor.writer = wId
	monitor.displayAttendanceList()
}

func (monitor *Monitor) removeWriter(wId WriterId) {
	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()

	monitor.writer = nullWriter
	monitor.displayAttendanceList()
}

// Reader process

func reader(id ReaderId, monitor *Monitor) {
	defer waitGroup.Done()

	for {
		randomSleep()
		monitor.rwMutex.RLock()
		monitor.addReader(id)
		randomSleep()
		monitor.removeReader(id)
		monitor.rwMutex.RUnlock()
	}
}

// Writer process

func writer(id WriterId, monitor *Monitor) {
	defer waitGroup.Done()

	for {
		randomSleep()
		monitor.rwMutex.Lock()
		monitor.addWriter(id)
		randomSleep()
		monitor.removeWriter(id)
		monitor.rwMutex.Unlock()
	}
}

// main

func main() {
	monitor := newMonitor()

	for i := 0; i < numReadersInt; i++ {
		waitGroup.Add(1)
		go reader(ReaderId(i), monitor)
	}

	for i := 0; i < numWritersInt; i++ {
		waitGroup.Add(1)
		go writer(WriterId(i), monitor)
	}

	waitGroup.Wait()
}
