package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func WhatTheCommit(client *http.Client) string {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get("http://whatthecommit.com/index.txt")
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	return strings.TrimSpace(string(body))
}

func sendCommitMessage(m Message, outgoing chan<- Message) {
	m.Text = WhatTheCommit(nil)
	outgoing <- m
}

func sendCode(m Message, outgoing chan<- Message) {
	m.Text = "稍等"
	if rand.Intn(2) > 0 {
		m.Text += "，刚看到"
	}
	h := time.Now().In(loc).Hour()
	if h >= 18 && h <= 20 {
		m.Text += "，我在地铁上"
	}
	outgoing <- m
	time.Sleep(1 * time.Second)

	if rand.Intn(2) > 0 {
		m.Text = fmt.Sprintf("%d <@%s>", rand.Intn(9000)+1000, m.User)
	} else {
		m.Text = fmt.Sprintf("<@%s> %d", m.User, rand.Intn(9000)+1000)
	}
	outgoing <- m
}

func isImage(m Message) bool {
	return m.SubType == "file_share" &&
		strings.HasPrefix(m.File.Mimetype, "image")
}

// at in the middle of the message is not supported
func isAt(text string) bool {
	return strings.HasPrefix(text, atId) || strings.HasSuffix(text, atId) ||
		strings.HasPrefix(text, alias) || strings.HasSuffix(text, alias)
}

func handleMessage(incoming <-chan Message, outgoing chan<- Message) {
	for msg := range incoming {
		if msg.Type != "message" {
			continue
		}
		if strings.Contains(msg.Text, "谢谢") {
			msg.Text = "不客气 :blush:"
			outgoing <- msg
		} else if isAt(msg.Text) {
			fields := strings.Fields(msg.Text)
			if len(fields) == 1 {
				sendCode(msg, outgoing)
			} else {
				var commit bool
				log.Println(fields)
				for _, f := range fields {
					if isAt(f) {
						continue
					}
					if strings.Contains(f, "commit") {
						commit = true
					}
				}
				if commit {
					sendCommitMessage(msg, outgoing)
				} else {
					if rand.Intn(2) > 0 {
						msg.Text = "呵呵"
					} else {
						msg.Text = "嘻嘻"
					}
					outgoing <- msg
				}
			}
		} else if isImage(msg) {
			sendCode(msg, outgoing)
		}
	}
}

func init() {
	log.Println("common init")
	loc, _ = time.LoadLocation("Asia/Shanghai")
	rand.Seed(time.Now().Unix())
}
