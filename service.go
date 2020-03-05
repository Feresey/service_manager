package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
)

//go:generate enumer -text -type MessageType,State --output service_enumer.go $GOFILE

type MessageType int

const (
	// You should close ServiceMessage chanel after receiving StateFinished or StateFailed
	MessageState MessageType = iota
	MessageString
)

type ServiceMessage struct {
	Name  string
	Type  MessageType
	State State
	Value string
}

type State int

const (
	StateDead State = iota
	StateStarted
	StateRunning
	StateFinished
	StateFailed
)

var execCommand = exec.CommandContext

type Service struct {
	Name    string
	Args    []string
	Command string
	State   State
	Err     error

	channel       chan ServiceMessage
	runningRegexp *regexp.Regexp
	output        io.ReadCloser
	ctx           context.Context
	cancel        context.CancelFunc
	cmd           *exec.Cmd
}

func NewService(name string, command string, args []string, runningTemplate *regexp.Regexp) *Service {
	return &Service{
		Name:          name,
		Command:       command,
		Args:          args,
		runningRegexp: runningTemplate,
	}
}

func (s *Service) Start(ctx context.Context) chan ServiceMessage {
	// Ну и нахйя ты передаешь контекст?
	// if ctx == nil {
	// 	ctx = context.Background()
	// }
	ctx, cancel := context.WithCancel(ctx)

	s.ctx = ctx
	s.cancel = cancel

	s.cmd = execCommand(ctx, s.Command, s.Args...)
	stdout, err := s.cmd.StdoutPipe()
	s.channel = make(chan ServiceMessage, 3) // we should handle one error that can occur during initialization and started and running messages
	s.setStarted()

	if err != nil {
		s.setFailed(err)
		return s.channel
	}

	if err := s.cmd.Start(); err != nil {
		s.setFailed(err)
		return s.channel
	}

	s.output = stdout

	go s.poll()

	return s.channel
}

func (s *Service) Stop() {
	//if s.State == StateStarted || s.State == StateRunning {
	err := s.cmd.Process.Signal(os.Interrupt)
	if err != nil {
		log.Print("Error stopping processes: ", err)
	}
	//s.cancel()
	//}
}

func (s *Service) setFailed(err error) {
	s.State = StateFailed
	s.Err = err
	s.channel <- ServiceMessage{
		Name:  s.Name,
		Type:  MessageState,
		State: StateFailed,
		Value: err.Error(),
	}

	close(s.channel)
}

func (s *Service) setStarted() {
	s.State = StateStarted
	s.channel <- ServiceMessage{
		Name:  s.Name,
		Type:  MessageState,
		State: StateStarted,
	}

	if s.runningRegexp == nil {
		s.State = StateRunning
		s.channel <- ServiceMessage{
			Name:  s.Name,
			Type:  MessageState,
			State: StateRunning,
		}
	}
}

func (s *Service) setFinished() {
	s.State = StateFinished
	s.channel <- ServiceMessage{
		Name:  s.Name,
		Type:  MessageState,
		State: StateFinished,
	}

	close(s.channel)
}

func (s *Service) setRunning() {
	s.State = StateRunning
	s.channel <- ServiceMessage{
		Name:  s.Name,
		Type:  MessageState,
		State: StateRunning,
	}
}

func (s *Service) handleIncomeString(input string) {
	if s.State == StateStarted && s.runningRegexp != nil {
		if s.runningRegexp.MatchString(input) {
			s.setRunning()
		}
	}
	s.channel <- ServiceMessage{
		Name:  s.Name,
		Type:  MessageString,
		Value: input,
	}
}

func (s *Service) poll() {
	scanner := bufio.NewScanner(s.output)

	for scanner.Scan() {
		s.handleIncomeString(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		s.setFailed(err)
		return
	}

	if err := s.cmd.Wait(); err != nil {
		s.setFailed(err)
		return
	}

	s.setFinished()
}

func isStartedState(s State) bool {
	return s == StateStarted || s == StateRunning
}
