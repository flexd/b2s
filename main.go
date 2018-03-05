package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/flexd/b2s/ircbot"
	"github.com/flexd/b2s/slackbot"
	"github.com/nlopes/slack"
	"github.com/spf13/viper"
)

var urlRegexp = regexp.MustCompile(`<([^>]*)\|([^>]*)>`)

func main() {
	viper.SetConfigName("config")    // name of config file (without extension)
	viper.AddConfigPath("/etc/b2s/") // path to look for the config file in
	viper.AddConfigPath(".")         // optionally look for config in the working directory
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	users := make(map[string]int)

	ircToSlack := make(map[string]string)
	slackToirc := make(map[string]string)
	bridgeCfg := viper.GetStringSlice("bridges")
	var ircChannels []string
	for _, pair := range bridgeCfg {
		sp := strings.Split(pair, ":")
		slackChannel := sp[0]
		ircChannel := sp[1]
		ircToSlack[ircChannel] = slackChannel
		slackToirc[slackChannel] = ircChannel
		ircChannels = append(ircChannels, ircChannel)
	}
	// Setup Slack
	slackb := slackbot.New(viper.GetString("slack.token"))
	slackEvents, err := slackb.Start()

	// Setup IRC
	ircConfig := viper.Sub("irc")
	ircb := ircbot.New(ircConfig, ircChannels)
	ircEvents, err := ircb.Start()
	if err != nil {
		panic(fmt.Errorf("Fatal error connecting to IRC%s \n", err))
	}
	// Handle incoming events
	for {
		select {
		// Handle IRC events
		case ev := <-ircEvents:
			switch ev.Code {
			case "PRIVMSG":
				target := ev.Arguments[0]
				if val, ok := ircToSlack[target]; ok {
					slackb.SendMessage(ev.Nick, val, ev.Message())
					incUser(users, ev.Nick, val)
				}
			case "CTCP_ACTION":
				target := ev.Arguments[0]
				if val, ok := ircToSlack[target]; ok {
					slackb.SendAction(ev.Nick, val, ev.Message())
					incUser(users, ev.Nick, val)
				}
			}
		// Handle Slack events!
		case msg := <-slackEvents:
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				// Ignore hello

			case *slack.ConnectedEvent:
				fmt.Println("Infos:", ev.Info)
				fmt.Println("Connection counter:", ev.ConnectionCount)

			case *slack.MessageEvent:
				name := ""
				isbot := false
				if ev.Username == "" {
					name, isbot, err = slackb.GetUsername(ev.User)
					if err != nil && !isbot {
						fmt.Println("Error getting username: ", err)
						continue
					}
				} else {
					name = ev.Username
				}

				channel, err := slackb.GetChannelName(ev.Channel)
				if err != nil {
					fmt.Println("Error getting channel name: ", err)
					continue
				}
				channel = "#" + channel
				if shouldHandle(users, name, channel) {
					if val, ok := slackToirc[channel]; ok {
						text := strings.Replace(ev.Text, "\n", " ", -1)
						text = strings.Replace(text, "\n", " ", -1)

						text = urlRegexp.ReplaceAllString(text, `$1 ($2)`)
						text = html.UnescapeString(text)

						if ev.Msg.SubType == "" {
							ircb.Connection.Privmsg(val, fmt.Sprintf("<%s>: %s", name, ev.Text))
						} else if ev.Msg.SubType == "me_message" {
							ircb.Connection.Action(val, fmt.Sprintf("<%s>: %s", name, ev.Text))
						}
					}
				}

			case *slack.PresenceChangeEvent:
				fmt.Printf("Presence Change: %v\n", ev)

			case *slack.LatencyReport:
				fmt.Printf("Current latency: %v\n", ev.Value)

			case *slack.RTMError:
				fmt.Printf("Error: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				fmt.Printf("Invalid credentials")
				return
			}
		}
	}
	close(ircEvents)
	close(slackEvents)
}

// Semaphores to manages messages

func incUser(users map[string]int, user, channel string) {
	key := user + channel
	if val, ok := users[key]; ok {
		users[key] = val + 1
	} else {
		users[key] = 1
	}
}

func shouldHandle(users map[string]int, user, channel string) bool {
	key := user + channel
	if val, ok := users[key]; ok {
		if val > 0 {
			users[key] = val - 1
			return false
		}
	}

	return true
}
