package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	normal = iota
	noCommand
	unknowCommand
	invalidArgument
	unexpectedError = 10
)

func helperCommandContext(ctx context.Context, t *testing.T, omit string, s ...string) (cmd *exec.Cmd) {
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
		return helperCommandContext(ctx, t, "", s...)
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
				os.Exit(invalidArgument)
			}

			fmt.Println(strings.ReplaceAll(args[0], ",", "\n"))
			args = args[1:]
		case "sleep":
			if len(args) == 0 {
				fmt.Println("No argument")
				os.Exit(invalidArgument)
			}

			m, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Println("Argument to sleep must be number!")
				os.Exit(invalidArgument)
			}

			time.Sleep(time.Millisecond * time.Duration(m))
			args = args[1:]
		case "error":
			os.Exit(unexpectedError)
		}
	}
}

func TestHelperService(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt)

	go func() {
		for range c {
			os.Exit(normal)
		}
	}()

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
		os.Exit(noCommand)
	}

	switch args[0] {
	case "service":
		execService(args[1:])
	default:
		fmt.Printf("Unknow command %s", args[0])

		os.Exit(unknowCommand)
	}

	os.Exit(normal)
}

func TestServiceStartSimple(t *testing.T) {
	defer setHelperCommand(t)()

	service := NewService("SIMPLE", "service", []string{}, nil)
	messages := service.Start(context.TODO())
	recorded := []ServiceMessage{}

	for message := range messages {
		recorded = append(recorded, message)
	}

	assert.Equal(t, []ServiceMessage{
		{
			Name:  "SIMPLE",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "SIMPLE",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "SIMPLE",
			Type:  MessageState,
			State: StateFinished,
		},
	}, recorded)
}
func TestServiceStartError(t *testing.T) {
	defer setHelperCommand(t)()

	service := NewService("ERROR", "service", []string{"error"}, nil)
	messages := service.Start(context.TODO())
	recorded := []ServiceMessage{}

	for message := range messages {
		recorded = append(recorded, message)
	}

	assert.Equal(t, []ServiceMessage{
		{
			Name:  "ERROR",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "ERROR",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "ERROR",
			Type:  MessageState,
			State: StateFailed,
			Value: "exit status 10",
		},
	}, recorded)
}

func TestServiceStartedRunningFinished(t *testing.T) {
	defer setHelperCommand(t)()

	startTemplate := regexp.MustCompile("ready")
	service := NewService("CAT", "service", []string{"lines", "hello,ready,exit"}, startTemplate)
	messages := service.Start(context.TODO())
	recorded := []ServiceMessage{}

	for message := range messages {
		recorded = append(recorded, message)
	}

	assert.Equal(t, []ServiceMessage{
		{
			Name:  "CAT",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "CAT",
			Type:  MessageString,
			Value: "hello",
		},
		{
			Name:  "CAT",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "CAT",
			Type:  MessageString,
			Value: "ready",
		},
		{
			Name:  "CAT",
			Type:  MessageString,
			Value: "exit",
		},
		{
			Name:  "CAT",
			Type:  MessageState,
			State: StateFinished,
		},
	}, recorded)
}
func TestServiceStop(t *testing.T) {
	defer setHelperCommand(t)()

	startTemplate := regexp.MustCompile("ready")
	service := NewService("ERROR", "service", []string{"lines", "ready", "sleep", "10000", "lines", "extra"}, startTemplate)
	messages := service.Start(context.TODO())
	recorded := []ServiceMessage{}

	for message := range messages {
		if message.Type == MessageState && message.State == StateRunning {
			//time.Sleep(time.Millisecond * 100)
			service.Stop()
		}

		recorded = append(recorded, message)
	}

	assert.Equal(t, []ServiceMessage{
		{
			Name:  "ERROR",
			Type:  MessageState,
			State: StateStarted,
		},
		{
			Name:  "ERROR",
			Type:  MessageState,
			State: StateRunning,
		},
		{
			Name:  "ERROR",
			Type:  MessageString,
			Value: "ready",
		},
		{
			Name:  "ERROR",
			Type:  MessageState,
			State: StateFinished,
		},
	}, recorded)
}
