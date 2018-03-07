package slackbot

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
)

type Bot struct {
	api        *slack.Client
	RTM        *slack.RTM
	events     chan *slack.RTMEvent
	channelMap map[string]string
	userMap    map[string]string
}

func New(token string) *Bot {
	api := slack.New(token)
	return &Bot{
		api:        api,
		events:     make(chan *slack.RTMEvent),
		RTM:        api.NewRTM(),
		channelMap: make(map[string]string),
		userMap:    make(map[string]string),
	}
}
func (b *Bot) Start() (chan *slack.RTMEvent, error) {
	go b.RTM.ManageConnection()
	go func() {
		for msg := range b.RTM.IncomingEvents {
			b.events <- &msg
		}
	}()
	return b.events, nil
}
func (b *Bot) SendMessage(from, channel, text string) {
	params := slack.PostMessageParameters{
		Username: from,
	}
	b.api.PostMessage(channel, text, params)
}
func (b *Bot) SendAction(from, channel, text string) {
	params := slack.PostMessageParameters{
		Username: from,
		Markdown: true,
	}
	b.api.PostMessage(channel, fmt.Sprintf("_%s_", text), params)
}
func (bot *Bot) GetChannelName(channelId string) (string, error) {
	if val, ok := bot.channelMap[channelId]; ok {
		return val, nil
	}

	info, err := bot.api.GetChannelInfo(channelId)
	if err != nil {
		fmt.Println("Could not fetch channel info")
		return "", fmt.Errorf("Could not fetch channel name: %s", err.Error())
	}

	bot.channelMap[channelId] = info.Name

	return info.Name, nil
}

func (bot *Bot) GetUsername(userId string) (name string, isbot bool, err error) {
	isbot = false

	if val, ok := bot.userMap[userId]; ok {
		name = val
		return
	}

	info, err := bot.api.GetUserInfo(userId)
	if err != nil {
		fmt.Println("Could not fetch user info")
		err = fmt.Errorf("Could not get username: %s", err.Error())
		return
	}

	if info.IsBot {
		isbot = true
		return
	}

	bot.userMap[userId] = info.Name
	name = info.Name
	return
}
func (bot *Bot) prependType(needle, msg, match, name string) string {
	// channels start with C
	if needle == "C" {
		msg = strings.Replace(msg, match, "#"+name, -1)

		// username starts with U
	} else if needle == "U" {
		msg = strings.Replace(msg, match, "@"+name, -1)

	}
	return msg

}

func (bot *Bot) resolveNames(id, needle, msg, match string) (string, error) {
	// channel starts with C
	if needle == "C" {
		name, err := bot.GetChannelName(id)
		if err != nil {
			return "", fmt.Errorf("Could not get channel name for %v, error: %v", id, err)
		}
		msg = strings.Replace(msg, match, "#"+name, -1)

		// username starts with U
	} else if needle == "U" {
		name, _, err := bot.GetUsername(id)
		if err != nil {
			return "", fmt.Errorf("Could not get username for %v, error: %v", id, err)
		}
		msg = strings.Replace(msg, match, "@"+name, -1)
	}
	return msg, nil

}
func (bot *Bot) PrettifyMessage(msg string) string {
	re := regexp.MustCompile("<(.*?)>")
	matches := re.FindAllString(msg, -1)

	for _, match := range matches {
		splits := strings.Split(match, "|")
		id := splits[0][1:]
		id = id[1 : len(id)-1] // remove the trailing >
		needle := id[:1]

		if len(splits) == 2 {
			// username of channel inside the text
			name := splits[1]
			name = name[:len(name)-1] // remove the trailing >

			msg = bot.prependType(needle, msg, match, name)
		} else if len(splits) == 1 {
			// need to fetch channel/username
			var err error
			msg, err = bot.resolveNames(id, needle, msg, match)
			if err != nil {
				fmt.Println(err)
				return string(match[0])
			}
		}
	}
	// Match @U19J5UPEC (flexd) var ikke mentions fikset her da?  :/
	re2 := regexp.MustCompile("([@#].+) \\((.+)\\)")
	matches2 := re2.FindAllStringSubmatch(msg, -1)

	for _, match := range matches2 {

		id := match[1]
		needle := id[1:2]

		if len(match) == 3 {
			// username of channel inside the text
			name := match[2]
			msg = bot.prependType(needle, msg, match[0], name)
		} else if len(match) == 2 {
			// need to fetch channel/username
			var err error
			msg, err = bot.resolveNames(id, needle, msg, match[0])
			if err != nil {
				fmt.Println(err)
				return string(match[0])
			}
		}

	}

	return msg
}
