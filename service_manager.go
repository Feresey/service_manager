package main

import (
	"regexp"
	"sync"
)

type TaskType int

const (
	TaskStart TaskType = iota
	TaskStop
	TaskExit
)

type TaskMessage struct {
	Name string
	Task TaskType
}

type ServiceManager struct {
	services     map[string]*Service
	requirements map[string][]string
	output       chan ServiceMessage
	merged       chan ServiceMessage
	taskChannel  chan TaskMessage
	states       map[string]State
	wgMerge      sync.WaitGroup

	// When poll exited
	pollDone chan struct{}
}

func NewServiceManager() *ServiceManager {
	sm := &ServiceManager{
		services:     make(map[string]*Service),
		requirements: make(map[string][]string),
		output:       make(chan ServiceMessage),
		merged:       make(chan ServiceMessage),
		pollDone:     make(chan struct{}),
		taskChannel:  make(chan TaskMessage),
		states:       make(map[string]State),
	}
	return sm
}

func (sm *ServiceManager) Register(name string,
	cmd string,
	args []string,
	running *regexp.Regexp,
	requirements []string,
) {
	sm.services[name] = NewService(name, cmd, args, running)
	sm.requirements[name] = requirements
	sm.states[name] = StateDead
}

func (sm *ServiceManager) Init() (chan ServiceMessage, error) {
	// TODO: make checks about requirements:
	// Graph is acyclic
	// All requirements does exists
	go sm.poll()
	return sm.merged, nil
}

// Start starts registered service. You should call all Register before first Start
func (sm *ServiceManager) Start(name string) {
	sm.taskChannel <- TaskMessage{
		Name: name,
		Task: TaskStart,
	}
}

func (sm *ServiceManager) Stop(name string) {
	sm.taskChannel <- TaskMessage{
		Name: name,
		Task: TaskStop,
	}
}

func (sm *ServiceManager) Close() {
	sm.taskChannel <- TaskMessage{
		Task: TaskExit,
	}
	sm.wgMerge.Wait()
	close(sm.output)
	close(sm.merged)
	close(sm.taskChannel)
}

func (sm *ServiceManager) poll() {
	tasks := []TaskMessage{}
	changed := make(map[string]struct{})
	isExiting := false
loop:
	for {
		select {
		case task := <-sm.taskChannel:
			switch task.Task {
			case TaskStart:
				if isExiting || isStartedState(sm.states[task.Name]) {
					continue loop
				}
			case TaskStop:
				if !isStartedState(sm.states[task.Name]) {
					continue loop
				}
			case TaskExit:
				isExiting = true
				// TODO exit
			}

			tasks = append(tasks, task)
		case message := <-sm.merged:
			sm.output <- message
			if message.Type == MessageState {
				sm.states[message.Name] = message.State
			}
			if message.Type != MessageState {
				continue loop
			}
		}
		if sm.applyTask(tasks[0], changed) {
			tasks = tasks[1:]
		}

		if len(tasks) == 0 && isExiting {
			break loop
		}
	}
	sm.wgMerge.Wait()
}

var schedulesFunc = map[TaskType]func(root string, states map[string]State, requirements map[string][]string) []string{
	TaskExit: func(root string, states map[string]State, requirements map[string][]string) []string {
		return GetEnabledLeafs(states, requirements)
	},
	TaskStop:  GetEnabledLeafsFromRoot,
	TaskStart: GetDisabledLeafsFromRoot,
}

func (sm *ServiceManager) applyTask(task TaskMessage, changed map[string]struct{}) bool {
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
	case TaskExit:
	}

	return tasks
}

func (sm *ServiceManager) stopAllServices() {
	serviceToStop := GetOrphanedStartedServices(sm.states, sm.requirements)
	if len(serviceToStop) == 0 {
		return
	}
	toStop := make(map[string]struct{})
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

// A: B C
// C: D
// B: D E
// E: F
//
func (sm *ServiceManager) startService(name string) bool {
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
		sm.wgMerge.Add(1)
		go func() {
			for message := range serviceChan {
				sm.merged <- message
			}
			sm.wgMerge.Done()
		}()
	}
	return false
}

func (sm *ServiceManager) stopService(name string) bool {
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
