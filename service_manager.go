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
	states         map[ServiceName]State

	// When poll exited

	stopAll  chan struct{}
	pollDone chan struct{}
}

func NewServiceManager() *ServiceManager {
	sm := &ServiceManager{
		services:       make(map[ServiceName]*Service),
		requirements:   make(map[ServiceName][]ServiceName),
		outputChannels: make(map[ServiceName]chan ServiceMessage),
		stopAll:        make(chan struct{}),
		pollDone:       make(chan struct{}),
		taskChannel:    make(chan TaskMessage),
		stateChannel:   make(chan StateMessage),
		states:         make(map[ServiceName]State),
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
	sm.states[name] = StateDead

	outputChannel := make(chan ServiceMessage)
	sm.outputChannels[name] = outputChannel
	return outputChannel
}

// Start starts registered service. You should call all Register before first Start
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
	sm.stopAll <- struct{}{}
	<-sm.pollDone
	/*
		for _, ch := range sm.outputChannels {
			close(ch)
		}
	*/
	close(sm.pollDone)
	close(sm.stateChannel)
	close(sm.stopAll)
	close(sm.taskChannel)
}

func (sm *ServiceManager) poll() {
	tasks := []TaskMessage{}
loop:
	for {
		select {
		case task := <-sm.taskChannel:
			switch task.Task {
			case TaskStart:
				if isStartedState(sm.states[task.Name]) {
					continue loop
				}
			case TaskStop:
				if !isStartedState(sm.states[task.Name]) {
					continue loop
				}
			}

			tasks = append(tasks, task)
		case state := <-sm.stateChannel:
			sm.states[state.Name] = state.State
		case <-sm.stopAll:
			sm.stopAllServices()
			sm.pollDone <- struct{}{}
			return

		}
		tasks = sm.applyFirstTask(tasks)
	}
}

func (sm *ServiceManager) stopAllServices() {
	serviceToStop := GetOrphanedStartedServices(sm.states, sm.requirements)
	if len(serviceToStop) == 0 {
		return
	}
	toStop := make(map[ServiceName]struct{})
	for _, name := range serviceToStop {
		toStop[name] = struct{}{}
		sm.stopService(name)
	}
loop:
	for {
		select {
		case <-sm.taskChannel:
			continue loop
		case state := <-sm.stateChannel:
			sm.states[state.Name] = state.State
			if !isStartedState(state.State) {
				delete(toStop, state.Name)
			}

		}
		for name, _ := range toStop {
			sm.stopService(name)
		}
		if len(toStop) == 0 {
			return
		}
	}

}

func (sm *ServiceManager) applyFirstTask(tasks []TaskMessage) []TaskMessage {
	if len(tasks) == 0 {
		return tasks
	}
	task := tasks[0]
	switch task.Task {
	case TaskStart:
		if isStartedState(sm.states[task.Name]) == true {
			return tasks[1:]
		}
		if sm.startService(task.Name) == true {
			return tasks[1:]
		}
	case TaskStop:
		if !isStartedState(sm.states[task.Name]) == true {
			return tasks[1:]
		}
		if sm.stopService(task.Name) == true {
			return tasks[1:]
		}
	}
	return tasks
}

// A: B C
// C: D
// B: D E
// E: F
//
func (sm *ServiceManager) startService(name ServiceName) bool {
	for _, requirement := range sm.requirements[name] {
		if sm.startService(requirement) == false {
			return false
		}
	}
	if sm.states[name] == StateRunning {
		return true
	}
	if !isStartedState(sm.states[name]) {
		serviceChan := sm.services[name].Start(nil)
		sm.states[name] = StateStarted
		go servicePoll(name, serviceChan, sm.outputChannels[name], sm.stateChannel)
	}
	return false
}

func (sm *ServiceManager) stopService(name ServiceName) bool {
	for _, requirement := range sm.requirements[name] {
		if sm.stopService(requirement) == false {
			return false
		}
	}
	if !isStartedState(sm.states[name]) {
		return true
	}
	if isStartedState(sm.states[name]) {
		sm.services[name].Stop()
	}
	return false
}

func servicePoll(name ServiceName, serviceChan chan ServiceMessage, forwardChan chan ServiceMessage, managerChan chan StateMessage) {
	for message := range serviceChan {
		if message.Type == MessageState {
			managerChan <- StateMessage{
				Name:  name,
				State: message.State,
			}
			if message.State == StateFinished || message.State == StateFailed {
				close(serviceChan)
			}
		}
		forwardChan <- message
	}
}
