package main

import (
	"context"
	"fmt"
	"regexp"
)

func poll(messages chan ServiceMessage) {
	for message := range messages {
		switch message.Type {
		case MessageState:
			if message.State == StateFailed || message.State == StateFinished {
				close(messages)
			}
		case MessageString:
		}

		fmt.Printf("%+v\n", message)
	}
}

func main() {
	template := regexp.MustCompile("started")
	service := NewService("test input", "cat", []string{"text.txt"}, template)
	messages := service.Start(context.TODO())
	poll(messages)
}
