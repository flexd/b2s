package relay

import (
	"fmt"
	"html"
	"log"
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
				switch ev.SubType {
				case "file_comment", "file_share":
					log.Printf("%+v\n", ev)
					//2018/04/18 13:12:49 Someone commented on a file: &{Msg:{Type:message Channel:C9KJHEZ3R User: Text:<@U19J5UPEC> commented on <@U19J5UPEC>’s file <https://bitraf.slack.com/files/U19J5UPEC/FA8RNMBQS/30174552_10160070492875532_2145625917_o.jpg|30174552_10160070492875532_2145625917_o.jpg>: boop Timestamp:1524049969.000276 ThreadTimestamp: IsStarred:false PinnedTo:[] Attachments:[] Edited:<nil> LastRead: Subscribed:false UnreadCount:0 SubType:file_comment Hidden:false DeletedTimestamp: EventTimestamp:1524049969.000276 BotID: Username: Icons:<nil> Inviter: Topic: Purpose: Name: OldName: Members:[] ReplyCount:0 Replies:[] ParentUserId: File:0xc4206c2600 Upload:false Comment:0xc42059cdc0 ItemType: ReplyTo:0 Team:T03G9SFJ7 Reactions:[] ResponseType: ReplaceOriginal:false} SubMessage:<nil>}

					var name string
					isbot := false
					var user string
					if ev.SubType == "file_comment" {
						var fileCommentRe = regexp.MustCompile(`<@(.+)> commented on <@(.+)>’s file <(.+)>: (.+)`)

						matches := fileCommentRe.FindStringSubmatch(ev.Text)

						if len(matches) == 0 {
							log.Println("Failed to extract details from file comment event")
							continue
						}
						user = matches[1]
					} else if ev.SubType == "file_share" {
						log.Println("ev.URLPrivate is:", ev.File.URLPrivate)
						var fileShareRe = regexp.MustCompile(`<(@.+)> uploaded a file.+<(.+)>`)
						matches := fileShareRe.FindStringSubmatch(ev.Text)

						if len(matches) == 0 {
							log.Println("Failed to extract details from file share event")
							continue
						}
						user = matches[1]
					}
					if ev.Username == "" {
						name, isbot, err = r.slackb.GetUsername(user)
						if err != nil && !isbot {
							fmt.Println("Error converting user string to userinfo:", user, "error message:", err)
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
				default:
					name := ""
					isbot := false
					if ev.Username == "" {
						name, isbot, err = r.slackb.GetUsername(ev.User)
						if err != nil && !isbot {
							fmt.Println("Error getting username:", ev.User, "error message:", err, "event:", ev)
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

							prefix := fmt.Sprintf("<%s> ", ircbot.F(name).Attr(ircbot.Bold).Fg(ircbot.Yellow).String())
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
	if subtype == "me_message" {
		r.ircb.Connection.Action(target, text)
	} else {
		r.ircb.Connection.Privmsg(target, text)
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
