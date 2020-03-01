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

var scheduleFuncMap = map[TaskType]func(root string, states map[string]State, requirements map[string][]string) []string{
	TaskExit: func(root string, states map[string]State, requirements map[string][]string) []string {
		return GetEnabledLeafs(states, requirements)
	},
	TaskStop:  GetEnabledLeafsFromRoot,
	TaskStart: GetDisabledLeafsFromRoot,
}

func (sm *ServiceManager) applyTask(task TaskMessage, changed map[string]struct{}) bool {
	scheduleFunc := scheduleFuncMap[task.Task]
	schedule := scheduleFunc(task.Name, sm.states, sm.requirements)
	// filter schedule to get what we should activate
	n := 0
	for _, x := range schedule {
		if _, ok := changed[x]; ok == false {
			schedule[n] = x
			n++
		}
	}
	schedule = schedule[:n]
	if len(schedule) == 0 {
		return true
	}

	taskFunc := sm.stopService
	if task.Task == TaskStart {
		taskFunc = sm.startService
	}

	for _, name := range schedule {
		taskFunc(name)
	}
	return false
}

func (sm *ServiceManager) startService(name string) {
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
}

func (sm *ServiceManager) stopService(name string) {
	if isStartedState(sm.states[name]) {
		sm.services[name].Stop()
	}
}
