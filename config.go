package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// The Config options
type Config struct {
	GitHubToken     string
	UpstreamOwner   string
	UpstreamRepo    string
	DownstreamOwner string
	DownstreamRepo  string
	RepoPath        string
	ToolPath        string
	UseWebhook      bool
	WebhookPort     int
	WebhookSecret   string
}

// Init will initalize the config file
func (c Config) Init() Config {
	if _, err := os.Stat("./config.json"); err != nil {
		fmt.Println("A configuration could not be found at config.json. Writing a config now. Please customize it to your needs and start the application again.")
		c.Save("./config.json")
		os.Exit(10)
	}

	return c.Load("./config.json")
}

// Save writes a config file at the specified path
func (c Config) Save(path string) {
	file, err := os.OpenFile(path, os.O_CREATE, 600)
	if err != nil {
		var errMsg = fmt.Sprintf("Error while opening config: %s\n", err)
		panic(errMsg)
	}
	defer file.Close()

	jsonString, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		var errMsg = fmt.Sprintf("Error while serializing config: %s\n", err)
		panic(errMsg)
	}

	file.Write(jsonString)
}

//Load loads a config file from the specified path
func (c Config) Load(path string) (config Config) {
	jsonString, err := ioutil.ReadFile(path)
	if err != nil {
		var errMsg = fmt.Sprintf("Error while opening config: %s\n", err)
		panic(errMsg)
	}

	err = json.Unmarshal(jsonString, &config)
	if err != nil {
		var errMsg = fmt.Sprintf("Error while deserializing config: %s\n", err)
		panic(errMsg)
	}

	return config
}
