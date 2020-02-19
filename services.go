package main

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"regexp"
)

type MessageType int

const (
	// You should close ServiceMessage chanel after receiving StateFinished or StateFailed
	MessageState MessageType = iota
	MessageString
)

type ServiceMessage struct {
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

func NewService(Name, Command string, Args []string, runningTemplate *regexp.Regexp) *Service {
	return &Service{
		Name:          Name,
		Command:       Command,
		Args:          Args,
		runningRegexp: runningTemplate,
	}
}

func (s *Service) Start(ctx context.Context) chan ServiceMessage {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	s.ctx = ctx
	s.cancel = cancel

	s.cmd = exec.CommandContext(ctx, s.Command, s.Args...)
	stdout, err := s.cmd.StdoutPipe()
	s.channel = make(chan ServiceMessage, 1) // we should handle one error that can occur during initialization
	if err != nil {
		s.setFailed(err)
		return s.channel
	}
	if err := s.cmd.Start(); err != nil {
		s.setFailed(err)
		return s.channel
	}
	s.output = stdout
	s.setStarted()

	go s.poll()
	return s.channel
}

func (s *Service) Stop() {
	if s.State == StateStarted || s.State == StateRunning {
		s.cancel()
	}
}

func (s *Service) setFailed(err error) {
	s.State = StateFailed
	s.Err = err
	s.channel <- ServiceMessage{
		Type:  MessageState,
		State: StateFailed,
		Value: err.Error(),
	}
}

func (s *Service) setStarted() {
	if s.runningRegexp == nil {
		s.State = StateRunning
		return
	}
	s.State = StateStarted
	s.channel <- ServiceMessage{
		Type:  MessageState,
		State: StateStarted,
	}
}

func (s *Service) setFinished() {
	s.State = StateFinished
	s.channel <- ServiceMessage{
		Type:  MessageState,
		State: StateFinished,
	}
}

func (s *Service) setRunning() {
	s.State = StateRunning
	s.channel <- ServiceMessage{
		Type:  MessageState,
		State: StateRunning,
	}
}

func (s *Service) handleIncomeString(input string) {
	if s.State == StateStarted && s.runningRegexp != nil {
		if s.runningRegexp.MatchString(input) == true {
			s.setRunning()
		}
	}
	s.channel <- ServiceMessage{
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
