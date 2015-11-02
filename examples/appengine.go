// +build appengine

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/hsluo/slack-bot"

	"google.golang.org/appengine"
	l "google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type task struct {
	context context.Context
	url     string
	data    url.Values
}

var (
	botId, atId string
	loc         *time.Location
	outgoing    chan task
)

func handleHook(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		return
	}

	token := req.PostFormValue("token")
	if token != credentials.HookToken {
		return
	}

	replyHook(req)
}

func replyHook(req *http.Request) {
	c := appengine.NewContext(req)
	l.Infof(c, "%v", req.Form)

	channel := req.PostFormValue("channel_id")
	text := req.PostFormValue("text")
	user_id := req.PostFormValue("user_id")

	client := urlfetch.Client(c)
	data := url.Values{"channel": {channel}}

	if strings.Contains(text, "commit") {
		data.Add("text", WhatTheCommit(client))
		outgoing <- task{context: c, url: slack.ChatPostMessageApi, data: data}
	} else if strings.Contains(text, bot.User) ||
		strings.Contains(text, bot.UserId) {
		d1 := url.Values{"channel": {channel}, "text": {"稍等"}}
		outgoing <- task{context: c, url: slack.ChatPostMessageApi, data: d1}

		text := codeWithAt(user_id)
		d2 := url.Values{"channel": {channel}, "text": {text}}
		outgoing <- task{context: c, url: slack.ChatPostMessageApi, data: d2}
	} else if strings.Contains(text, "谢谢") {
		data.Add("text", "不客气 :blush:")
		outgoing <- task{context: c, url: slack.ChatPostMessageApi, data: data}
	} else {
		if rand.Intn(2) > 0 {
			data.Add("text", "呵呵")
		} else {
			data.Add("text", "嘻嘻")
		}
		outgoing <- task{context: c, url: slack.ChatPostMessageApi, data: data}
	}
}

func worker(outgoing chan task) {
	for task := range outgoing {
		_, err := bot.WithClient(urlfetch.Client(task.context)).PostForm(task.url, task.data)
		if err != nil {
			l.Errorf(task.context, "%s\n%v", err, task.data)
		}
	}
}

func standUpAlert(rw http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)
	url := credentials.SlackbotUrl
	if url == "" {
		l.Errorf(c, "no slackbot URL provided")
		return
	}
	url = fmt.Sprintf("%s&channel=%%23%s", url, "general")
	client := urlfetch.Client(c)
	client.Post(url, "text/plain", strings.NewReader("stand up"))
}

func logglyAlert(rw http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)

	attachment, err := NewAttachment(req)
	if err != nil {
		l.Errorf(c, "%s", err)
		return
	}

	bytes, err := json.Marshal([]slack.Attachment{attachment})
	if err != nil {
		l.Errorf(c, "%s", err)
		return
	}
	data := url.Values{}
	data.Add("channel", "#loggly")
	data.Add("attachments", string(bytes))
	data.Add("as_user", "false")
	outgoing <- task{context: c, url: slack.ChatPostMessageApi, data: data}
}

func replyCommit(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(rw, WhatTheCommit(urlfetch.Client(appengine.NewContext(req))))
}

var domain = os.Getenv("LOGGLY_DOMAIN")
var logglyClient = &LogglyClient{
	username: os.Getenv("LOGGLY_USERNAME"),
	password: os.Getenv("LOGGLY_PASSWORD"),
}

func logglySearch(rw http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)
	logglyClient.client = urlfetch.Client(ctx)

	api := fmt.Sprintf("http://%s.loggly.com/apiv2/search?%s",
		domain,
		url.Values{
			"q":    {`syslog.severity:"Error" OR syslog.severity:"Warning" OR json.status:>=500`},
			"from": {"-10m"},
		}.Encode())
	rsidResp := make(map[string]interface{})
	logglyClient.Request(api).UnmarshallJson(&rsidResp)

	rsid := rsidResp["rsid"].(map[string]interface{})["id"].(string)
	api = fmt.Sprintf("http://%s.loggly.com/apiv2/events?rsid=%s", domain, rsid)
	searchResult := SearchResult{}
	logglyClient.Request(api).UnmarshallJson(&searchResult)
	l.Infof(ctx, "rsid=%v events=%v", rsid, searchResult.TotalEvents)

	for _, e := range searchResult.Events {
		text := e.Logmsg
		if strings.Contains(e.Logmsg, "#012") {
			text = fmtHit(e.Logmsg)
		}
		data := url.Values{}
		data.Add("channel", "#loggly")
		data.Add("text", text)
		data.Add("as_user", "false")
		outgoing <- task{context: ctx, url: slack.ChatPostMessageApi, data: data}
	}
}

func init() {
	log.Println("appengine init")
	outgoing = make(chan task)
	go worker(outgoing)

	http.HandleFunc("/hook", handleHook)
	http.HandleFunc("/alerts/standup", standUpAlert)
	//http.HandleFunc("/loggly", logglyAlert)
	http.HandleFunc("/loggly/search", logglySearch)
	http.HandleFunc("/cmds/whatthecommit",
		slack.ValidateCommand(http.HandlerFunc(replyCommit), credentials.Commands))
}

func main() {}