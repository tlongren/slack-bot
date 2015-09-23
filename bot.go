package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

var (
	token              string
	botId, atId        string
	incoming, outgoing chan Message
)

func sendCommitMessage(m Message, outgoing chan<- Message) {
	resp, err := http.Get("http://whatthecommit.com/index.txt")
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	m.Text = strings.TrimSpace(string(body))
	outgoing <- m
}

func sendCode(m Message, outgoing chan<- Message) {
	m.Text = "稍等"
	if rand.Intn(2) > 0 {
		m.Text += "，刚看到"
	}
	if time.Now().Hour() > 6 {
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
func isAt(m Message) bool {
	return strings.HasPrefix(m.Text, atId) || strings.HasSuffix(m.Text, atId)
}

func handleMessage(incoming <-chan Message, outgoing chan<- Message) {
	for msg := range incoming {
		if msg.Type != "message" {
			continue
		}
		if strings.Contains(msg.Text, "谢谢") {
			msg.Text = "不客气 :blush:"
			outgoing <- msg
		} else if isAt(msg) {
			fields := strings.Fields(msg.Text)
			if len(fields) == 1 {
				sendCode(msg, outgoing)
			} else {
				var commit bool
				for _, f := range fields {
					if strings.Contains(f, atId) {
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

func readToken(file string) (token string) {
	b, err := ioutil.ReadFile("CREDENTIALS")
	if err != nil {
		log.Fatal(err)
	}
	token = strings.Split(string(b), "\n")[0]
	log.Println(token)
	return
}

func startServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("listening on " + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func init() {
	log.SetFlags(log.Lshortfile)
	rand.Seed(time.Now().Unix())
}

func main() {
	wsurl, id := rtmStart(readToken("CREDENTIALS"))
	botId = id
	atId = "<@" + botId + ">"
	log.Println(wsurl, botId)

	ws, err := websocket.Dial(wsurl, "", "https://api.slack.com/")
	if err != nil {
		log.Fatal(err)
	}

	incoming = make(chan Message)
	outgoing = make(chan Message)

	go rtmReceive(ws, incoming)
	go rtmSend(ws, outgoing)
	go handleMessage(incoming, outgoing)

	startServer()
}
