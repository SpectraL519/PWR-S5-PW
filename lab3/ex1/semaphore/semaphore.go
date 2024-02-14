package main

type Semaphore interface {
	Acquire()
	Release()
}

type semaphoreRecordType struct{}

type semaphoreChannelType chan semaphoreRecordType

type semaphore struct {
	channel semaphoreChannelType
}

func (s *semaphore) Acquire() {
	s.channel <- semaphoreRecordType{}
}

func (s *semaphore) Release() {
	<-s.channel
}

func NewSemaphore(capacity uint8) Semaphore {
	return &semaphore{
		channel: make(semaphoreChannelType, capacity),
	}
}

func NewBinarySemaphore() Semaphore {
	return &semaphore{
		channel: make(semaphoreChannelType, 1),
	}
}
