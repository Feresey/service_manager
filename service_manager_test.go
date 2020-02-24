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
	defer m.Close()
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
	println("stop")
}

func TestServiceManagerRestart(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	defer m.Close()
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
	defer m.Close()
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
	defer close(aMessages)
	bMessages := m.Register("B", "service", []string{}, nil, []ServiceName{"A"})
	defer close(bMessages)

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
	defer close(serviceMessages)
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
func TestServiceManagerStopWithDependency(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	defer m.Close()

	aMessages := m.Register("A", "service", []string{"sleep", "5000"}, nil, []ServiceName{})
	defer close(aMessages)
	startTemplate := regexp.MustCompile("ready")
	bMessages := m.Register("B", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []ServiceName{"A"})
	defer close(bMessages)

	wg := &sync.WaitGroup{}
	aStarts := int64(0)
	aStops := int64(0)

	m.Start("B")
	wg.Add(1)
	go func() {
		for message := range aMessages {
			if message.Type == MessageState && message.State == StateRunning {
				atomic.AddInt64(&aStarts, 1)
			}
			if message.Type == MessageState && message.State == StateFailed {
				t.Error("aService wasn't stopped gracefully")
				break
			}
			if message.Type == MessageState && message.State == StateFinished {
				atomic.AddInt64(&aStops, 1)
				break
			}
		}
		wg.Done()
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	wg.Add(1)
	go func() {
	loop:
		for {
			select {
			case <-ticker.C:
				t.Error("Service wasn't stopped after second")
				break loop
			case message := <-bMessages:
				if message.Type == MessageState && message.State == StateRunning {
					m.Stop("B")
				}
				if message.Type == MessageState && message.State == StateFailed {
					t.Error("bService wasn't stopped gracefully")
					break loop
				}
				if message.Type == MessageState && message.State == StateFinished {
					assert.Equal(t, int64(1), atomic.LoadInt64(&aStarts), "a wasn't started or was started more than once")
					assert.Equal(t, int64(1), atomic.LoadInt64(&aStops), "a wasn't stopped or was stopped more than once")
					break loop
				}
			}
		}
		wg.Done()
	}()
	wg.Wait()
}
func TestServiceManagerClose(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()

	startTemplate := regexp.MustCompile("ready")
	aMessages := m.Register("A", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []ServiceName{})
	defer close(aMessages)
	bMessages := m.Register("B", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []ServiceName{"A"})
	defer close(bMessages)
	cMessages := m.Register("C", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []ServiceName{})
	defer close(cMessages)

	wg := &sync.WaitGroup{}
	finished := &sync.WaitGroup{}
	var finishes int64 = 0

	waitForRunning := func(messages chan ServiceMessage) {
		for message := range messages {
			if message.Type == MessageState && message.State == StateRunning {
				wg.Done()
			}
			if message.Type == MessageState && message.State == StateFinished {
				atomic.AddInt64(&finishes, 1)
				break
			}
		}
		finished.Done()
	}

	m.Start("B")
	m.Start("C")
	wg.Add(1)
	finished.Add(1)
	go waitForRunning(aMessages)
	wg.Add(1)
	finished.Add(1)
	go waitForRunning(bMessages)
	wg.Add(1)
	finished.Add(1)
	go waitForRunning(cMessages)
	wg.Wait() // after this all services started
	m.Close()
	finished.Wait()
	if atomic.LoadInt64(&finishes) != 3 {
		t.Errorf("Finished must be 3, actual: %d", finishes)
	}
}
