package ircbot

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/thoj/go-ircevent"
)

type Bot struct {
	Connection *irc.Connection
	events     chan *irc.Event
	channels   []string
	config     *viper.Viper
}

//func New(server, nick, realname, password string) *Bot {
func New(cfg *viper.Viper, channels []string) *Bot {
	return &Bot{
		Connection: irc.IRC(cfg.GetString("nick"), cfg.GetString("realname")),
		events:     make(chan *irc.Event),
		config:     cfg,
		channels:   channels,
	}
}
func (b *Bot) Start() (chan *irc.Event, error) {
	b.Connection.VerboseCallbackHandler = false
	b.Connection.Debug = false
	b.Connection.AddCallback("001", func(e *irc.Event) {
		for _, c := range b.channels {
			b.Connection.Join(c)
		}
	})
	b.Connection.AddCallback("366", func(e *irc.Event) {})
	b.Connection.AddCallback("433", func(e *irc.Event) {
		panic("Nickname already in use!")
	})
	b.Connection.AddCallback("PRIVMSG", func(e *irc.Event) { b.events <- e })
	b.Connection.AddCallback("CTCP_ACTION", func(e *irc.Event) { b.events <- e })
	b.Connection.AddCallback("JOIN", func(e *irc.Event) { b.events <- e })
	b.Connection.AddCallback("PART", func(e *irc.Event) { b.events <- e })
	b.Connection.AddCallback("QUIT", func(e *irc.Event) { b.events <- e })
	err := b.Connection.Connect(b.config.GetString("server"))
	if err != nil {
		fmt.Printf("Err %s", err)
		return nil, err
	}
	go b.Connection.Loop()
	return b.events, nil
}
