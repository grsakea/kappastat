package main

import (
	"github.com/grsakea/kappastat/common"
	"github.com/sorcix/irc"
	"log"
	"time"
)

func setupChat() *IrcBot {
	conf := LoadConfig("config.json")

	b := &IrcBot{
		server: "irc.twitch.tv:6667",
		login:  conf.ChatLogin,
		pass:   conf.ChatPass,
	}

	err := b.connect()

	if err != nil {
		log.Panicln(err)
	}
	return b
}

func loopChat(c chan Message, infos chan kappastat.ChatEntry) {
	bot := setupChat()
	for {
		select {
		case msg, ok := <-bot.data:
			if !ok {
				log.Print("IRC bot failed")
				return
			}
			messageHandler(bot.writer, infos, msg)
		case msg, ok := <-c:
			if !ok {
				return
			}
			if msg.s == AddStream {
				addChannel(bot.writer, msg.v)
			} else if msg.s == RemoveStream {
				removeChannel(bot.writer, msg.v)
			}
		}
	}
}

func addChannel(s *irc.Encoder, name string) {
	s.Encode(&irc.Message{
		Command: irc.JOIN,
		Params:  []string{"#" + name},
	})
}

func removeChannel(s *irc.Encoder, name string) {
	s.Encode(&irc.Message{
		Command: irc.PART,
		Params:  []string{"#" + name},
	})
}

func messageHandler(s *irc.Encoder, infos chan kappastat.ChatEntry, m *irc.Message) {
	handled := make(map[string]bool)
	handled[irc.PING] = true
	handled[irc.PRIVMSG] = true
	handled[irc.RPL_WELCOME] = true
	handled[irc.RPL_YOURHOST] = true
	handled[irc.RPL_CREATED] = true
	handled[irc.RPL_MYINFO] = true
	handled[irc.RPL_MOTD] = true
	handled[irc.RPL_MOTDSTART] = true
	handled[irc.RPL_ENDOFMOTD] = true
	handled[irc.RPL_NAMREPLY] = true
	handled[irc.RPL_ENDOFNAMES] = true
	handled[irc.JOIN] = true

	if !handled[m.Command] {
		log.Println("Unhandled Message ", m.Command)
		return
	}
	if m.Command == irc.PING {
		PingHandler(s, m)
		return
	} else if m.Command == irc.PRIVMSG {
		PrivmsgHandler(infos, m)
	}
}

func PingHandler(s *irc.Encoder, m *irc.Message) {
	s.Encode(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})
}

func PrivmsgHandler(infos chan kappastat.ChatEntry, m *irc.Message) {
	channelName := m.Params[0][1:]
	sender := m.User
	text := m.Trailing
	infos <- kappastat.ChatEntry{channelName, sender, time.Now(), text}
}
