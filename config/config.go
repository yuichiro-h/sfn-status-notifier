package config

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var c Config

type StateMachineName string

type Config struct {
	Debug                bool        `yaml:"debug"`
	DynamoDBTable        string      `yaml:"dynamodb_table"`
	RegistrationInterval int         `yaml:"registration_interval"`
	WatchInterval        int         `yaml:"watch_interval"`
	Region               string      `yaml:"region"`
	Slack                SlackConfig `yaml:"slack"`
	StateMachines        map[StateMachineName]struct {
		Deadline *int        `yaml:"deadline"`
		Slack    SlackConfig `yaml:"slack"`
	} `yaml:"state_machines"`
}

type SlackConfig struct {
	ApiToken        string `yaml:"api_token"`
	Username        string `yaml:"username"`
	Channel         string `yaml:"channel"`
	AttachmentColor string `yaml:"attachment_color"`
	IconURL         string `yaml:"icon_url"`
}

func (c *SlackConfig) Merge(sc SlackConfig) {
	if sc.ApiToken != "" {
		c.ApiToken = sc.ApiToken
	}
	if sc.AttachmentColor != "" {
		c.AttachmentColor = sc.AttachmentColor
	}
	if sc.Channel != "" {
		c.Channel = sc.Channel
	}
	if sc.IconURL != "" {
		c.IconURL = sc.IconURL
	}
	if sc.Username != "" {
		c.Username = sc.Username
	}
}

func Load(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := yaml.Unmarshal(data, &c); err != nil {
		return errors.WithStack(err)
	}

	if c.RegistrationInterval == 0 {
		c.RegistrationInterval = 60
	}
	if c.WatchInterval == 0 {
		c.WatchInterval = 60
	}

	return nil
}

func Get() *Config {
	return &c
}
