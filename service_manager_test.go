package main

import (
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceManagerStart(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	serviceMessages := m.Register("TEST", "service", []string{"lines", "hello,ready"}, startTemplate, []ServiceName{})
	defer close(serviceMessages)
	m.Start("TEST")

	recorded := []ServiceMessage{}
	for message := range serviceMessages {
		recorded = append(recorded, message)
		if message.Type == MessageState && !isStartedState(message.State) {
			break
		}
	}
	assert.Equal(t, []ServiceMessage{
		ServiceMessage{
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Type:  1,
			Value: "hello",
		},
		ServiceMessage{
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Type:  1,
			Value: "ready",
		},
		ServiceMessage{
			Type:  0,
			State: StateFinished,
		},
	}, recorded)
}

func TestServiceManagerRestart(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	serviceMessages := m.Register("TEST", "service", []string{"lines", "hello,ready"}, startTemplate, []ServiceName{})
	defer close(serviceMessages)
	recorded := []ServiceMessage{}
	expected := []ServiceMessage{
		ServiceMessage{
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Type:  1,
			Value: "hello",
		},
		ServiceMessage{
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Type:  1,
			Value: "ready",
		},
		ServiceMessage{
			Type:  0,
			State: StateFinished,
		},
	}

	m.Start("TEST")
	for {
		message := <-serviceMessages
		recorded = append(recorded, message)
		if message.Type == MessageState && !isStartedState(message.State) {
			break
		}
	}
	assert.Equal(t, expected, recorded)
	recorded = recorded[:0]
	m.Start("TEST")
	for {
		message := <-serviceMessages
		recorded = append(recorded, message)
		if message.Type == MessageState && !isStartedState(message.State) {
			break
		}
	}
	assert.Equal(t, expected, recorded)
}
func TestServiceManagerStartWithDependency(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	aMessages := m.Register("A", "service", []string{"lines", "ready"}, startTemplate, []ServiceName{})
	defer close(aMessages)
	bMessages := m.Register("B", "service", []string{}, nil, []ServiceName{"A"})
	defer close(bMessages)

	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}
	isAStarted := false

	wg.Add(1)
	go func() {
		for m := range aMessages {
			if m.Type == MessageState && m.State == StateRunning {
				mu.Lock()
				isAStarted = true
				mu.Unlock()
			}
			if m.Type == MessageState && !isStartedState(m.State) {
				break
			}
		}
		wg.Done()
	}()

	m.Start("B")
	recorded := []ServiceMessage{}
	for message := range bMessages {
		recorded = append(recorded, message)
		if message.Type == MessageState && message.State == StateStarted {
			mu.Lock()
			dependencyFulfilled := isAStarted
			mu.Unlock()
			if dependencyFulfilled == false {
				t.Error("B started before A!")
			}
		}
		if message.Type == MessageState && !isStartedState(message.State) {
			break
		}
	}
	assert.Equal(t, []ServiceMessage{
		ServiceMessage{
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Type:  0,
			State: StateFinished,
		},
	}, recorded)
	wg.Wait()
}
func TestServiceManagerStartWithFullfilledDependency(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	aMessages := m.Register("A", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []ServiceName{})
	bMessages := m.Register("B", "service", []string{}, nil, []ServiceName{"A"})

	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}
	aStarts := 0

	m.Start("A")
	wg.Add(1)
	go func() {
		for m := range aMessages {
			if m.Type == MessageState && m.State == StateRunning {
				mu.Lock()
				aStarts++
				mu.Unlock()
				wg.Done()
			}
			if m.Type == MessageState && !isStartedState(m.State) {
				break
			}
		}
	}()
	wg.Wait()

	m.Start("B")
	recorded := []ServiceMessage{}
	for message := range bMessages {
		recorded = append(recorded, message)
		if message.Type == MessageState && message.State == StateStarted {
			mu.Lock()
			dependencyStartedOnce := aStarts == 1
			mu.Unlock()
			if dependencyStartedOnce == false {
				t.Error("B started more than once!")
			}
		}
		if message.Type == MessageState && !isStartedState(message.State) {
			break
		}
	}
	assert.Equal(t, []ServiceMessage{
		ServiceMessage{
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Type:  0,
			State: StateFinished,
		},
	}, recorded)
	m.Close()
}
