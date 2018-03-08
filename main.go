package main

import (
	"fmt"

	"github.com/flexd/b2s/ircbot"
	"github.com/flexd/b2s/relay"
	"github.com/flexd/b2s/slackbot"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")    // name of config file (without extension)
	viper.AddConfigPath(".")         // optionally look for config in the working directory
	viper.AddConfigPath("/etc/b2s/") // path to look for the config file in
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	// Setup Slack
	slackb := slackbot.New(viper.GetString("slack.token"))

	ircb := ircbot.New(viper.Sub("irc"), viper.GetStringSlice("bridges"))
	// Setup the relay
	relay := relay.New(viper.GetStringSlice("bridges"), ircb, slackb)
	// engage the relay!
	relay.Loop()
}
