package main

import (
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	aStarts := int64(0)

	wg.Add(1)
	go func() {
		for m := range aMessages {
			if m.Type == MessageState && m.State == StateRunning {
				atomic.AddInt64(&aStarts, 1)

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
			if atomic.LoadInt64(&aStarts) != 1 {
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
	aStarts := int64(0)

	m.Start("A")
	wg.Add(1)
	go func() {
		for m := range aMessages {
			if m.Type == MessageState && m.State == StateRunning {
				atomic.AddInt64(&aStarts, 1)
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
			if atomic.LoadInt64(&aStarts) != 1 {
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

func TestServiceManagerStop(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	defer m.Close()

	startTemplate := regexp.MustCompile("ready")
	serviceMessages := m.Register("TEST", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []ServiceName{})
	m.Start("TEST")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ticker.C:
			t.Error("Service wasn't stopped after second")
			break loop
		case message := <-serviceMessages:
			if message.Type == MessageState && message.State == StateRunning {
				m.Stop("TEST")

			}
			if message.Type == MessageState && message.State == StateFailed {
				t.Error("Service wasn't stopped gracefully")
				break loop
			}
			if message.Type == MessageState && message.State == StateFinished {
				break loop
			}
		}
	}
}
