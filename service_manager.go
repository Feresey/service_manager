package main

import (
	"regexp"
)

type TaskType int

const (
	TaskStart TaskType = iota
	TaskStop
)

type TaskMessage struct {
	Name ServiceName
	Task TaskType
}

type StateMessage struct {
	Name  ServiceName
	State State
}

type ServiceManager struct {
	services       map[ServiceName]*Service
	requirements   map[ServiceName][]ServiceName
	outputChannels map[ServiceName]chan ServiceMessage
	taskChannel    chan TaskMessage
	stateChannel   chan StateMessage

	// When poll exited

	pollDone chan struct{}
}

func NewServiceManager() *ServiceManager {
	sm := &ServiceManager{
		services:     make(map[ServiceName]*Service),
		requirements: make(map[ServiceName][]ServiceName),
		pollDone:     make(chan struct{}),
		taskChannel:  make(chan TaskMessage),
		stateChannel: make(chan StateMessage),
	}
	go sm.poll()
	return sm
}

func (sm *ServiceManager) Register(name ServiceName,
	cmd string,
	args []string,
	running *regexp.Regexp,
	requirements []ServiceName,
) chan ServiceMessage {
	sm.services[name] = NewService(name, cmd, args, running)
	sm.requirements[name] = requirements

	outputChannel := make(chan ServiceMessage)
	sm.outputChannels[name] = outputChannel
	return outputChannel
}

func (sm *ServiceManager) Start(name ServiceName) {
	sm.taskChannel <- TaskMessage{
		Name: name,
		Task: TaskStart,
	}
}

func (sm *ServiceManager) Stop(name ServiceName) {
	sm.taskChannel <- TaskMessage{
		Name: name,
		Task: TaskStop,
	}
}

func (sm *ServiceManager) Close() {
	// TODO:
	// Wait when pollDone closed
}

func (sm *ServiceManager) poll() {
	states := map[ServiceName]State{}
	for message := range sm.stateChannel {
	}
}
