package main

import (
	"fmt"
	"github.com/sorcix/irc"
)

type Session struct {
	*irc.Conn
	// Set this bit on construction
	Server   string
	Port     int
	UserName string
	NickName string
	RealName string
	// Set by the internals
	messages <-chan *irc.Message
}

func (session *Session) Dial() error {
	conn, err := irc.Dial(fmt.Sprintf("%s:%d", session.Server, session.Port))
	if err != nil {
		return err
	}
	session.Conn = conn
	return session.handshake()
}

func (session *Session) handshake() (err error) {
	err = session.Encode(&irc.Message{
		Command: "NICK",
		Params:  []string{session.NickName},
	})
	if err != nil {
		return err
	}
	return session.Encode(&irc.Message{
		Command:  "USER",
		Params:   []string{session.UserName, "0", "*"},
		Trailing: session.RealName,
	})
}

func (session *Session) Privmsg(name, message string) error {
	return session.Encode(&irc.Message{
		Command:  "PRIVMSG",
		Params:   []string{name},
		Trailing: message,
	})
}

func (session *Session) Quit(reason string) error {
	if reason == "" {
		reason = "Shutting down"
	}
	defer session.Close()
	return session.Encode(&irc.Message{
		Command:  "QUIT",
		Trailing: reason,
	})
}

func (session *Session) handlePing(message *irc.Message) (err error) {
	// Ha ha ha this is so dodgy
	message.Command = "PONG"
	return session.Encode(message)
}

var toIgnore = [...]string{
	"002",
	"003",
	"004",
	"005",
	"251",
	"252",
	"254",
	"255",
	"265",
	"266",
	"376",
	"422",
}

func (session *Session) readPump() (err error) {
	var shouldIgnore = make(map[string]bool, len(toIgnore))
	for _, num := range toIgnore {
		shouldIgnore[num] = true
	}
	var m *irc.Message

	for {
		m, err = session.Decode()
		if err != nil {
			return err
		}
		// Ignore informative spam
		if shouldIgnore[m.Command] {
			continue
		}
		// Start of glorious message type switches
		if m.Command == "PING" {
			log.Debug("Got ping: %s", m.Trailing)
			err = session.handlePing(m)
			if err != nil {
				log.Error("Couldn't pong: %s", err)
			}
		} else if m.Command == "001" {
			log.Info("Connection established")
			setupNickserv(session)
		} else if m.Command == "ERROR" {
			log.Critical("Server hung up: %s", m.Trailing)
			return nil
		} else {
			log.Debug("Got unhandled message: %+v", m)
		}
	}
}
