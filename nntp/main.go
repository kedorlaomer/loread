package nntp

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type state struct {
	groups         []string             // subscribed groups
	paths          map[MessageId]string // maps message ids to their paths
	deleteMessages []MessageId          // messages to be deleted
	messages       map[*Container]bool  // messages in current group
	group          string               // group currently being visited
}

var exit = make(chan bool, 0)

func Main() {
	conf, err := ReadConfig("config.txt")

	if err != nil {
		panic(err)
	}

	FetchArticles(conf)
	groups := strings.Split(conf["groups"], ", ")

	s := state{
		groups:         groups,
		paths:          make(map[MessageId]string),
		group:          "",
		deleteMessages: make([]MessageId, 0),
	}

	http.Handle("/", &s)
	go func() {
		err = http.ListenAndServe(":8080", nil)
		log.Fatal(err)
	}()

	<-exit
	time.Sleep(time.Second * time.Duration(3))
}

func (s *state) ServeHTTP(out http.ResponseWriter, request *http.Request) {
	v := request.URL.Query()

	// if there's a delete request, add to delete list
	del, ok := v["delete"]

	if ok && len(del) > 0 {
		s.deleteMessages = append(s.deleteMessages, MessageId(del[0]))
	}

	// Serve. The action depends on view.
	switch operation, ok := v["view"]; {
	case !ok || len(operation) == 0:
		fallthrough
	case operation[0] == "overview":
		InitialScreen(s.groups, out)

	case operation[0] == "group":
		group, ok := v["arg"]
		if !ok || len(group) == 0 {
			ErrorPageF(out, "no arg provided in query %s", request.URL.String())
		}

		raw, paths, err := GetArticles(group[0])

		if err != nil {
			ErrorPage(err, out)
			break
		}

		articles := make([]ParsedArticle, len(raw))

		for i := range raw {
			articles[i] = FormatArticle(raw[i])
			s.paths[articles[i].Id] = paths[i]
		}

		containers := Thread(articles)
		s.messages = containers
		s.group = group[0]
		GroupOverview(group[0], containers, out)

	case operation[0] == "article":
		arg, ok := v["arg"]

		if !ok || len(arg) == 0 {
			ErrorPageF(out, "no arg provided in query %s", request.URL.String())
		}

		id := MessageId(arg[0])
		container := findArticle(s.messages, id)
		if container == nil || container.Article == nil {
			ErrorPageF(out, "article with id '%s' not found in query %s", id, request.URL.String())
		} else {
			ShowArticle(container, s.group, out)
		}

	case operation[0] == "quit":
		// delete s.deleteMessages, ignore errors
		for _, id := range s.deleteMessages {
			path := s.paths[id]
			os.Remove(path)
		}

		// good bye!
		FinalScreen(out)

		exit <- true

	default:
		log.Fatalf("unknown view: %s", operation[0])
	}
}

func findArticle(containers map[*Container]bool, id MessageId) *Container {
	q := NewQueue()
	for c := range containers {
		q.Enqueue(c)
	}

	for !q.Empty() {
		c := q.Dequeue().(*Container)
		if c == nil {
			continue
		}

		if c.Next != nil {
			q.Enqueue(c.Next)
		}

		if c.Child != nil {
			q.Enqueue(c.Child)
		}

		if c.Article != nil && c.Article.Id == id {
			return c
		}
	}

	return nil
}
