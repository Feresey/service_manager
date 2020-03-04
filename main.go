package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"

	"github.com/rivo/tview"
	"gopkg.in/yaml.v2"
)

type ServiceConfig struct {
	Command      []string
	Requirements []string
	RunTemplate  string
}

func addService(manager *ServiceManager, name string, config *ServiceConfig) error {
	if config == nil {
		return errors.New("service config is empty")
	}
	if name == "" {
		return errors.New("name should not be empty")
	}
	if len(config.Command) < 1 {
		return errors.New("command should contain at least one element")
	}
	var runTemplate *regexp.Regexp
	if config.RunTemplate != "" {
		var err error
		runTemplate, err = regexp.Compile(config.RunTemplate)
		if err != nil {
			return fmt.Errorf("failed to compile run_template regexp: %w", err)
		}
	}
	err := manager.Register(
		name,
		config.Command[0],
		config.Command[1:],
		runTemplate,
		config.Requirements,
	)
	if err != nil {
		return err
	}
	return nil
}

func parseConfig(manager *ServiceManager, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	configs := map[string]*ServiceConfig{}
	if err := yaml.NewDecoder(file).Decode(&configs); err != nil {
		return err
	}
	for name, config := range configs {
		if err := addService(manager, name, config); err != nil {
			return fmt.Errorf("Failed to parse service config for %s: %w", name, err)
		}
	}
	return nil
}

var configPath = flag.String("config", "config.yaml", "path to config file")

func main() {
	flag.Parse()

	manager := NewServiceManager()
	if err := parseConfig(manager, *configPath); err != nil {
		fmt.Printf("Error while parsing config: %s\n", err)
		os.Exit(1)
	}
	messages, services, err := manager.Init()
	if err != nil {
		fmt.Printf("Error while initing service manager: %s", err)
		os.Exit(1)
	}
	_ = messages

	app := tview.NewApplication()

	serviceList := tview.NewList().ShowSecondaryText(false)
	serviceList.SetBorder(true)
	for _, name := range services {
		serviceList.AddItem("[red]● [-]"+name, "", 0, func() {})
	}
	pages := tview.NewPages().SetBorder(true)
	input := tview.NewInputField()

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(pages, 0, 3, false).
			AddItem(serviceList, 0, 1, false),
			0, 1, false).
		AddItem(input, 1, 1, false)
	if err := app.SetRoot(flex, true).SetFocus(flex).Run(); err != nil {
		panic(err)
	}
}
