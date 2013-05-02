package nntp

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

type state struct {
	groups         []string             // subscribed groups
	paths          map[MessageId]string // maps message ids to their paths
	deleteMessages []MessageId          // messages to be deleted
	messages       []*Container         // messages in current group
	group          string               // group currently being visited
}

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

	fmt.Println("listening")
	http.Handle("/", &s)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *state) ServeHTTP(out http.ResponseWriter, request *http.Request) {
	v := request.URL.Query()

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

	default:
		log.Fatalf("unknown view: %s", operation[0])
	}
}

func findArticle(containers []*Container, id MessageId) *Container {
	q := NewQueue()
	for _, c := range containers {
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
