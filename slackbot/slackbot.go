package slackbot

import (
	"fmt"

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
