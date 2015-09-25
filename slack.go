package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"appengine"
	"appengine/urlfetch"
)

type Message struct {
	Type    string     `json:"type"`
	SubType string     `json:"subtype"`
	Channel string     `json:"channel"`
	User    string     `json:"user"`
	Text    string     `json:"text"`
	Ts      string     `json:"ts"`
	File    FileObject `json:"file"`
}

type FileObject struct {
	Mimetype   string `json:"mimetype"`
	Filetype   string `json:"filetype"`
	PrettyType string `json:"pretty_type"`
}

type Bot struct {
	Token, BotId string
	Context      appengine.Context
}

func NewBot(token string, c *http.Client) (b Bot, err error) {
	resp, err := c.PostForm("https://slack.com/api/auth.test", url.Values{"token": {token}})
	if err != nil {
		return
	}
	respAuthTest, err := asJson(resp)
	if err == nil {
		b = Bot{
			Token: token,
			BotId: respAuthTest["user_id"].(string),
		}
	}
	return
}

func (b Bot) WithCtx(c appengine.Context) Bot {
	b.Context = c
	return b
}

func (b Bot) PostForm(url string, data url.Values) (resp *http.Response, err error) {
	data.Add("token", b.Token)
	data.Add("as_user", "true")
	b.Context.Debugf("API=%s, data=%v", url, data)
	client := urlfetch.Client(b.Context)
	resp, err = client.PostForm(url, data)
	if err != nil {
		log.Println(err)
	}
	respJson, err := asJson(resp)
	if err != nil {
		return
	}
	log.Println(respJson)
	if !respJson["ok"].(bool) {
		err = errors.New(respJson["error"].(string))
	}
	return
}

func (b Bot) ChatPostMessage(data url.Values) {
	b.PostForm("https://slack.com/api/chat.postMessage", data)
}

func asJson(resp *http.Response) (m map[string]interface{}, err error) {
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	m = make(map[string]interface{})
	err = json.Unmarshal(body, &m)
	return
}

func (b Bot) Reply(hookData url.Values, c appengine.Context) {
	text := hookData["text"][0]
	if strings.Contains(text, "commit") {
		data := url.Values{
			"channel": {hookData["channel_id"][0]},
			"text":    {"I got a commit"},
		}
		b.ChatPostMessage(data)
	}
}

// Calls rtm.start API, return websocket url and bot id
func rtmStart(token string) (wsurl string, id string) {
	resp, err := http.PostForm("https://slack.com/api/rtm.start", url.Values{"token": {token}})
	if err != nil {
		log.Fatal(err)
	}
	respRtmStart, err := asJson(resp)
	if err != nil {
		log.Fatal(err)
	}
	wsurl = respRtmStart["url"].(string)
	id = respRtmStart["self"].(map[string]interface{})["id"].(string)
	return
}

func rtmReceive(ws *websocket.Conn, incoming chan<- Message) {
	for {
		var m Message
		if err := websocket.JSON.Receive(ws, &m); err != nil {
			log.Println(err)
		} else {
			log.Printf("read %v", m)
			incoming <- m
		}
	}
}

func rtmSend(ws *websocket.Conn, outgoing <-chan Message) {
	for m := range outgoing {
		m.User = botId
		m.Ts = fmt.Sprintf("%f", float64(time.Now().UnixNano())/1000000000.0)
		log.Printf("send %v", m)
		if err := websocket.JSON.Send(ws, m); err != nil {
			log.Println(err)
		}
	}
}
