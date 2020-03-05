package main

import (
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	waitTime = 5 * time.Second
	aStarted = 1
)

func TestServiceManagerStart(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	m.Register("TEST", "service", []string{"lines", "hello,ready"}, startTemplate, []string{})
	messages, err := m.Init()

	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	m.Start("TEST")

	recorded := make([]ServiceMessage, 0, len(messages))

	for message := range messages {
		recorded = append(recorded, message)

		if message.Type == MessageState && !isStartedState(message.State) {
			go m.Close()
		}
	}

	assert.Equal(t, []ServiceMessage{
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "TEST",
			Type:  MessageString,
			Value: "hello",
		},
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "TEST",
			Type:  MessageString,
			Value: "ready",
		},
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateFinished,
		},
	}, recorded)
}

func TestServiceManagerRestart(t *testing.T) {
	defer setHelperCommand(t)()

	expected := []ServiceMessage{
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "TEST",
			Type:  MessageString,
			Value: "hello",
		},
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "TEST",
			Type:  MessageString,
			Value: "ready",
		},
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateFinished,
		},
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "TEST",
			Type:  MessageString,
			Value: "hello",
		},
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "TEST",
			Type:  MessageString,
			Value: "ready",
		},
		{
			Name:  "TEST",
			Type:  MessageState,
			State: StateFinished,
		},
	}

	var (
		m             = NewServiceManager()
		startTemplate = regexp.MustCompile("ready")
		recorded      = make([]ServiceMessage, 0, len(expected))
		wasStopped    = false
	)

	m.Register("TEST", "service", []string{"lines", "hello,ready"}, startTemplate, []string{})

	messages, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	m.Start("TEST")

	for message := range messages {
		recorded = append(recorded, message)

		if message.Type == MessageState &&
			!isStartedState(message.State) {
			if !wasStopped {
				go m.Start("TEST")
				wasStopped = true

				continue
			}
		}

		go m.Close()
	}

	assert.Equal(t, expected, recorded)
}

func TestServiceManagerStartWithDependency(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	m.Register("A", "service", []string{"lines", "ready"}, startTemplate, []string{})
	m.Register("B", "service", []string{}, nil, []string{"A"})

	aStarts := int64(0)

	messages, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	m.Start("B")

	recorded := make([]ServiceMessage, 0, len(messages))

	for message := range messages {
		switch message.Name {
		case "A":
			if message.Type == MessageState && message.State == StateRunning {
				atomic.AddInt64(&aStarts, aStarted)
			}
		case "B":
			recorded = append(recorded, message)

			if message.Type == MessageState &&
				message.State == StateStarted {
				if atomic.LoadInt64(&aStarts) != aStarted {
					t.Error("B started before A!")
				}
			}

			if message.Type == MessageState && !isStartedState(message.State) {
				go m.Close()
			}
		}
	}

	assert.Equal(t, []ServiceMessage{
		{
			Name:  "B",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "B",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "B",
			Type:  MessageState,
			State: StateFinished,
		},
	}, recorded)
}

func TestServiceManagerStartWithFullfilledDependency(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	m.Register("A", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []string{})
	m.Register("B", "service", []string{"lines", "ready"}, startTemplate, []string{"A"})

	aStarts := int64(0)

	messages, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	m.Start("A")

	recorded := make([]ServiceMessage, 0, len(messages))

	for message := range messages {
		switch message.Name {
		case "A":
			if message.Type == MessageState && message.State == StateRunning {
				atomic.AddInt64(&aStarts, aStarted)

				go m.Start("B")
			}
		case "B":
			recorded = append(recorded, message)

			if message.Type == MessageState &&
				message.State == StateRunning {
				if atomic.LoadInt64(&aStarts) != aStarted {
					t.Error("A started more than once!")

					go m.Close()
				}
			}
		}
	}

	assert.Equal(t, []ServiceMessage{
		{
			Name:  "B",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "B",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "B",
			Type:  MessageString,
			Value: "ready",
		},
		{
			Name:  "B",
			Type:  MessageState,
			State: StateFinished,
		},
	}, recorded)
}

func TestServiceManagerStop(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")

	m.Register("TEST", "service", []string{"lines", "ready", "sleep", "10000"}, startTemplate, []string{})
	messages, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	m.Start("TEST")

	ticker := time.NewTicker(waitTime)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			t.Error("Service wasn't stopped after second")
			break loop
		case message, ok := <-messages:
			if ok == false {
				break loop
			}
			if message.Type == MessageState && message.State == StateRunning {
				go m.Stop("TEST")
			}
			if message.Type == MessageState && message.State == StateFailed {
				t.Error("Service wasn't stopped gracefully")

				go m.Close()
			}
			if message.Type == MessageState && message.State == StateFinished {
				go m.Close()
			}
		}
	}
}

func TestServiceManagerStopWithDependency(t *testing.T) {
	defer setHelperCommand(t)()

	var (
		m             = NewServiceManager()
		aStarts       = int64(0)
		aStops        = int64(0)
		ticker        = time.NewTicker(waitTime)
		startTemplate = regexp.MustCompile("ready")
	)

	m.Register("A", "service", []string{"sleep", "5000"}, nil, []string{})
	m.Register("B", "service", []string{"lines", "ready", "sleep", "10000"}, startTemplate, []string{"A"})

	messages, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	defer ticker.Stop()
	m.Start("B")

loop:
	for {
		select {
		case <-ticker.C:
			t.Error("Service wasn't stopped after second")

			break loop
		case message, ok := <-messages:
			if ok == false {
				break loop
			}
			switch message.Name {
			case "A":
				if message.Type == MessageState && message.State == StateRunning {
					atomic.AddInt64(&aStarts, aStarts)
				}

				if message.Type == MessageState && message.State == StateFailed {
					t.Error("aService wasn't stopped gracefully")
				}

				if message.Type == MessageState && message.State == StateFinished {
					atomic.AddInt64(&aStops, aStarts)
				}
			case "B":
				if message.Type == MessageState && message.State == StateRunning {
					go m.Stop("B")
				}

				if message.Type == MessageState && message.State == StateFailed {
					t.Error("bService wasn't stopped gracefully")
				}

				if message.Type == MessageState && message.State == StateFinished {
					assert.Equal(t, aStarts, atomic.LoadInt64(&aStarts), "a wasn't started or was started more than once")
					assert.Equal(t, aStarts, atomic.LoadInt64(&aStops), "a wasn't stopped or was stopped more than once")

					go m.Close()
				}
			}
		}
	}
}

func TestServiceManagerClose(t *testing.T) {
	defer setHelperCommand(t)()

	services := []struct {
		name string
		req  []string
	}{
		{
			name: "A",
			req:  []string{},
		},
		{
			name: "B",
			req:  []string{"A"},
		},
		{
			name: "C",
			req:  []string{},
		},
	}

	var (
		startTemplate       = regexp.MustCompile("ready")
		m                   = NewServiceManager()
		started             = &sync.WaitGroup{}
		finished            = make(chan struct{})
		finishes      int64 = 0
	)

	for _, s := range services {
		m.Register(s.name, "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, s.req)
	}

	started.Add(len(services))

	messages, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	go func() {
		for message := range messages {
			if message.Type == MessageState && message.State == StateRunning {
				started.Done()
			}

			if message.Type == MessageState && message.State == StateFinished {
				atomic.AddInt64(&finishes, 1)
			}
		}

		finished <- struct{}{}
	}()

	m.Start("B")
	m.Start("C")
	started.Wait()
	m.Close()
	<-finished

	if atomic.LoadInt64(&finishes) != int64(len(services)) {
		t.Errorf("Finished must be 3, actual: %d", finishes)
	}
}
