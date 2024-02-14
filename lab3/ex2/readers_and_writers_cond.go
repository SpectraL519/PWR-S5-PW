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
	readers     ReaderList
	readersCond *sync.Cond

	writer      WriterId
	writersCond *sync.Cond
}

func newMonitor() *Monitor {
	return &Monitor{
		readers:     make(ReaderList, 0),
		readersCond: sync.NewCond(&sync.Mutex{}),
		writer:      nullWriter,
		writersCond: sync.NewCond(&sync.Mutex{}),
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
	monitor.readersCond.L.Lock()
	defer monitor.readersCond.L.Unlock()

	for monitor.isWriterPresent() {
		monitor.readersCond.Wait()
	}

	monitor.readers = append(monitor.readers, rId)
	monitor.displayAttendanceList()
	monitor.readersCond.Signal()
}

func (monitor *Monitor) removeReader(rId ReaderId) {
	monitor.readersCond.L.Lock()
	defer monitor.readersCond.L.Unlock()

	for i, prId := range monitor.readers {
		if prId != rId {
			continue
		}

		monitor.readers = append(monitor.readers[:i], monitor.readers[i+1:]...)
		break
	}

	monitor.displayAttendanceList()

	if !monitor.areReadersPresent() {
		monitor.writersCond.Signal()
	}
}

func (monitor *Monitor) addWriter(wId WriterId) {
	monitor.writersCond.L.Lock()
	defer monitor.writersCond.L.Unlock()

	for monitor.isWriterPresent() || monitor.areReadersPresent() {
		monitor.writersCond.Wait()
	}

	monitor.writer = wId
	monitor.displayAttendanceList()
}

func (monitor *Monitor) removeWriter(wId WriterId) {
	monitor.writersCond.L.Lock()
	defer monitor.writersCond.L.Unlock()

	monitor.writer = nullWriter
	monitor.displayAttendanceList()
	monitor.readersCond.Signal()
	monitor.writersCond.Signal()
}

// Reader process

func reader(id ReaderId, monitor *Monitor) {
	defer waitGroup.Done()

	for {
		randomSleep()
		monitor.addReader(id)
		randomSleep()
		monitor.removeReader(id)
	}
}

// Writer process

func writer(id WriterId, monitor *Monitor) {
	defer waitGroup.Done()

	for {
		randomSleep()
		monitor.addWriter(id)
		randomSleep()
		monitor.removeWriter(id)
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
