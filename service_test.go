package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func helperCommandContext(t *testing.T, ctx context.Context, omit string, s ...string) (cmd *exec.Cmd) {
	cs := []string{"-test.run=TestHelperService", "--"}
	cs = append(cs, s...)
	if ctx != nil {
		cmd = exec.CommandContext(ctx, os.Args[0], cs...)
	} else {
		cmd = exec.Command(os.Args[0], cs...)
	}
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func setHelperCommand(t *testing.T) func() {
	oldExec := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		s := append([]string{name}, args...)
		return helperCommandContext(t, ctx, "", s...)
	}
	return func() {
		execCommand = oldExec
	}
}

func execService(args []string) {
	for len(args) > 0 {
		command := args[0]
		args = args[1:]
		switch command {
		case "lines":
			if len(args) == 0 {
				fmt.Println("No argument")
				os.Exit(3)
			}
			fmt.Println(strings.ReplaceAll(args[0], ",", "\n"))
			args = args[1:]
		case "sleep":
			if len(args) == 0 {
				fmt.Println("No argument")
				os.Exit(3)
			}
			m, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Println("Argument to sleep must be number!")
				os.Exit(3)
			}
			time.Sleep(time.Millisecond * time.Duration(m))
			args = args[1:]
		case "error":
			os.Exit(10)
		}

	}

}

func TestHelperService(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Println("No command")
		os.Exit(1)
	}
	switch args[0] {
	case "service":
		execService(args[1:])
	default:
		fmt.Printf("Unknow command %s", args[0])
		os.Exit(2)
	}
	defer os.Exit(0)
}

func TestServiceStartSimple(t *testing.T) {
	defer setHelperCommand(t)()

	service := NewService("SIMPLE", "service", []string{}, nil)
	messages := service.Start(nil)
	recorded := []ServiceMessage{}
	for message := range messages {
		if message.Type == MessageState && !isStartedState(message.State) {
			close(messages)
		}
		recorded = append(recorded, message)
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
}
func TestServiceStartError(t *testing.T) {
	defer setHelperCommand(t)()

	service := NewService("ERROR", "service", []string{"error"}, nil)
	messages := service.Start(nil)
	recorded := []ServiceMessage{}
	for message := range messages {
		if message.Type == MessageState && !isStartedState(message.State) {
			close(messages)
		}
		recorded = append(recorded, message)
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
			State: StateFailed,
			Value: "exit status 10",
		},
	}, recorded)
}

func TestServiceStartedRunningFinished(t *testing.T) {
	defer setHelperCommand(t)()

	startTemplate := regexp.MustCompile("ready")
	service := NewService("CAT", "service", []string{"lines", "hello,ready,exit"}, startTemplate)
	messages := service.Start(nil)
	recorded := []ServiceMessage{}
	for message := range messages {
		if message.Type == MessageState && !isStartedState(message.State) {
			close(messages)
		}
		recorded = append(recorded, message)
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
			Type:  1,
			Value: "exit",
		},
		ServiceMessage{
			Type:  0,
			State: StateFinished,
		},
	}, recorded)
}
func TestServiceStop(t *testing.T) {
	defer setHelperCommand(t)()

	startTemplate := regexp.MustCompile("ready")
	service := NewService("ERROR", "service", []string{"lines", "ready", "sleep", "10000", "lines", "extra"}, startTemplate)
	messages := service.Start(nil)
	recorded := []ServiceMessage{}
	for message := range messages {
		if message.Type == MessageState && !isStartedState(message.State) {
			close(messages)
		}
		if message.Type == MessageState && message.State == StateRunning {
			time.Sleep(time.Millisecond * 100)
			service.Stop()
		}
		recorded = append(recorded, message)
	}
	if len(recorded) == 0 {
		t.Fatal("Recordered zero messages from service")
	}
	recorded[len(recorded)-1].Value = ""
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
			Type:  1,
			Value: "ready",
		},
		ServiceMessage{
			Type:  0,
			State: StateFailed,
		},
	}, recorded)
}
