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
	m.Register("TEST", "service", []string{"lines", "hello,ready"}, startTemplate, []string{})
	messages, _, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}
	m.Start("TEST")

	recorded := []ServiceMessage{}
	for message := range messages {
		recorded = append(recorded, message)
		if message.Type == MessageState && !isStartedState(message.State) {
			go m.Close()
		}
	}
	assert.Equal(t, []ServiceMessage{
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  1,
			Value: "hello",
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  1,
			Value: "ready",
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateFinished,
		},
	}, recorded)
}

func TestServiceManagerRestart(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()
	startTemplate := regexp.MustCompile("ready")
	m.Register("TEST", "service", []string{"lines", "hello,ready"}, startTemplate, []string{})
	recorded := []ServiceMessage{}
	expected := []ServiceMessage{
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  1,
			Value: "hello",
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  1,
			Value: "ready",
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateFinished,
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  1,
			Value: "hello",
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  1,
			Value: "ready",
		},
		ServiceMessage{
			Name:  "TEST",
			Type:  0,
			State: StateFinished,
		},
	}

	messages, _, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}
	m.Start("TEST")
	wasStopped := false
	for message := range messages {
		recorded = append(recorded, message)
		if message.Type == MessageState && !isStartedState(message.State) {
			if wasStopped == false {
				go m.Start("TEST")
				wasStopped = true
				continue
			}
			go m.Close()
		}
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

	messages, _, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	recorded := []ServiceMessage{}
	m.Start("B")
	for message := range messages {
		switch message.Name {
		case "A":
			if message.Type == MessageState && message.State == StateRunning {
				atomic.AddInt64(&aStarts, 1)

			}
		case "B":
			recorded = append(recorded, message)
			if message.Type == MessageState && message.State == StateStarted {
				if atomic.LoadInt64(&aStarts) != 1 {
					t.Error("B started before A!")
				}
			}
			if message.Type == MessageState && !isStartedState(message.State) {
				go m.Close()
			}
		}
	}
	assert.Equal(t, []ServiceMessage{
		ServiceMessage{
			Name:  "B",
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Name:  "B",
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Name:  "B",
			Type:  0,
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

	messages, _, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	recorded := []ServiceMessage{}

	m.Start("A")
	for message := range messages {
		switch message.Name {
		case "A":
			if message.Type == MessageState && message.State == StateRunning {
				atomic.AddInt64(&aStarts, 1)
				go m.Start("B")
			}
		case "B":
			recorded = append(recorded, message)
			if message.Type == MessageState && message.State == StateRunning {
				if atomic.LoadInt64(&aStarts) != 1 {
					t.Error("A started more than once!")
				}
				go m.Close()
			}
		}
	}

	assert.Equal(t, []ServiceMessage{
		ServiceMessage{
			Name:  "B",
			Type:  0,
			State: StateStarted,
		},
		ServiceMessage{
			Name:  "B",
			Type:  0,
			State: StateRunning,
		},
		ServiceMessage{
			Name:  "B",
			Type:  1,
			Value: "ready",
		},
		ServiceMessage{
			Name:  "B",
			Type:  0,
			State: StateFinished,
		},
	}, recorded)
}

func TestServiceManagerStop(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()

	startTemplate := regexp.MustCompile("ready")

	m.Register("TEST", "service", []string{"lines", "ready", "sleep", "10000"}, startTemplate, []string{})
	messages, _, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	m.Start("TEST")

	ticker := time.NewTicker(time.Second * 5)
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

	m := NewServiceManager()

	m.Register("A", "service", []string{"sleep", "5000"}, nil, []string{})
	startTemplate := regexp.MustCompile("ready")
	m.Register("B", "service", []string{"lines", "ready", "sleep", "10000"}, startTemplate, []string{"A"})

	messages, _, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	aStarts := int64(0)
	aStops := int64(0)

	ticker := time.NewTicker(time.Second * 5)
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
					atomic.AddInt64(&aStarts, 1)
				}
				if message.Type == MessageState && message.State == StateFailed {
					t.Error("aService wasn't stopped gracefully")
				}
				if message.Type == MessageState && message.State == StateFinished {
					atomic.AddInt64(&aStops, 1)
				}
			case "B":
				if message.Type == MessageState && message.State == StateRunning {
					go m.Stop("B")
				}
				if message.Type == MessageState && message.State == StateFailed {
					t.Error("bService wasn't stopped gracefully")
				}
				if message.Type == MessageState && message.State == StateFinished {
					assert.Equal(t, int64(1), atomic.LoadInt64(&aStarts), "a wasn't started or was started more than once")
					assert.Equal(t, int64(1), atomic.LoadInt64(&aStops), "a wasn't stopped or was stopped more than once")
					go m.Close()
				}
			}
		}
	}

}

func TestServiceManagerClose(t *testing.T) {
	defer setHelperCommand(t)()

	m := NewServiceManager()

	startTemplate := regexp.MustCompile("ready")
	m.Register("A", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []string{})
	m.Register("B", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []string{"A"})
	m.Register("C", "service", []string{"lines", "ready", "sleep", "5000"}, startTemplate, []string{})

	started := &sync.WaitGroup{}
	finished := make(chan struct{})
	var finishes int64 = 0

	messages, _, err := m.Init()
	if err != nil {
		t.Fatal("can not init service manager: ", err)
	}

	started.Add(3)
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
	if atomic.LoadInt64(&finishes) != 3 {
		t.Errorf("Finished must be 3, actual: %d", finishes)
	}
}
