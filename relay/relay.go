package relay

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/flexd/b2s/ircbot"
	"github.com/flexd/b2s/slackbot"
	"github.com/nlopes/slack"
)

var urlRegexp = regexp.MustCompile(`<([^>]*)\|([^>]*)>`)

type Relay struct {
	// clients
	ircb   *ircbot.Bot
	slackb *slackbot.Bot
	// internal state
	users      map[string]int
	ircToSlack map[string]string
	slackToirc map[string]string
}

func New(bridgeCfg []string, ircb *ircbot.Bot, slackb *slackbot.Bot) *Relay {
	ircToSlack := make(map[string]string)
	slackToirc := make(map[string]string)
	// Loop through the stupid format to build our state
	for _, pair := range bridgeCfg {
		sp := strings.Split(pair, ":")
		slackChannel := sp[0]
		ircChannel := sp[1]
		ircToSlack[ircChannel] = slackChannel
		slackToirc[slackChannel] = ircChannel
	}
	return &Relay{
		users:      make(map[string]int),
		ircToSlack: ircToSlack,
		slackToirc: slackToirc,
		ircb:       ircb,
		slackb:     slackb,
	}
}
func (r *Relay) Loop() {
	slackEvents, err := r.slackb.Start()
	if err != nil {
		panic(fmt.Errorf("Fatal error setting up Slack connection%s \n", err))
	}
	ircEvents, err := r.ircb.Start()
	if err != nil {
		panic(fmt.Errorf("Fatal error connecting to IRC%s \n", err))
	}
	defer func() {
		close(slackEvents)
		close(ircEvents)
		fmt.Println("Relay loop finished :)")
	}()
	// Handle incoming events
	for {
		select {
		// Handle IRC events
		case ev := <-ircEvents:
			switch ev.Code {
			case "PRIVMSG":
				target := ev.Arguments[0]
				if val, ok := r.ircToSlack[target]; ok {
					r.slackb.SendMessage(ev.Nick, val, ev.Message())
					r.incUser(ev.Nick, val)
				}
			case "CTCP_ACTION":
				target := ev.Arguments[0]
				if val, ok := r.ircToSlack[target]; ok {
					r.slackb.SendAction(ev.Nick, val, ev.Message())
					r.incUser(ev.Nick, val)
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
					name, isbot, err = r.slackb.GetUsername(ev.User)
					if err != nil && !isbot {
						fmt.Println("Error getting username:", ev.User, "error message:", err)
						continue
					}
				} else {
					name = ev.Username
				}

				channel, err := r.slackb.GetChannelName(ev.Channel)
				if err != nil {
					fmt.Println("Error getting channel name: ", err)
					continue
				}
				channel = "#" + channel
				if r.shouldHandle(name, channel) {
					if target, ok := r.slackToirc[channel]; ok {
						text := strings.Replace(ev.Text, "\n", " ", -1)
						text = strings.Replace(text, "\n", " ", -1)

						text = urlRegexp.ReplaceAllString(text, `$1 ($2)`)
						text = r.slackb.PrettifyMessage(text)
						text = r.slackb.ConvertSmileys(text)
						text = html.UnescapeString(text)

						prefix := fmt.Sprintf("<%s> ", name)
						plen := len(prefix)
						for len(text) > 400-plen {
							index := 400
							for i := index - plen; i >= index-plen-15; i-- {
								if string(text[i]) == " " {
									index = i
									break
								}
							}
							r.Reply(ev.Msg.SubType, target, fmt.Sprintf("%s%s", prefix, text[:index]))
							text = text[index:]
						}
						r.Reply(ev.Msg.SubType, target, fmt.Sprintf("%s%s", prefix, text))
					}
				}

			case *slack.PresenceChangeEvent:
				// Ignore these
			case *slack.LatencyReport:
				// Ignore these
			case *slack.RTMError:
				fmt.Printf("Error: %s\n", ev.Error())

			case *slack.InvalidAuthEvent:
				fmt.Printf("Invalid credentials")
				return
			}
		}
	}
}

func (r *Relay) Reply(subtype, target, text string) {
	if subtype == "" {
		r.ircb.Connection.Privmsg(target, text)
	} else if subtype == "me_message" {
		r.ircb.Connection.Action(target, text)
	}
}

// Semaphores to manage relayed messages

func (r *Relay) incUser(user, channel string) {
	key := user + channel
	if val, ok := r.users[key]; ok {
		r.users[key] = val + 1
	} else {
		r.users[key] = 1
	}
}

func (r *Relay) shouldHandle(user, channel string) bool {
	key := user + channel
	if val, ok := r.users[key]; ok {
		if val > 0 {
			r.users[key] = val - 1
			return false
		}
	}
	return true
}
