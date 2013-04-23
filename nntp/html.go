package nntp

import (
	"fmt"
	"html/template"
	"io"
	"net/url"
	"strings"
)

// Produces HTML for an initial screen listing all subscribed
// groups.
func InitialScreen(config map[string]string, out io.Writer) {
	groups := strings.Split(config["groups"], ", ")
	template1 :=
		`<html>
    <head>
        <title>Loread — The low reader</title>
    </head>
    <body>
        <h1>Your subscribed groups<h1>
        <ul>
            {{range .}}
                <li><a href="?arg={{.}}&view=group">{{.}}</a></li>
            {{else}}
                Nothing?
            {{end}}
        </ul>
    </body>
</html>`
	tmpl := template.Must(template.New("initial").Parse(template1))
	err := tmpl.Execute(out, groups)

	if err != nil {
		panic(err)
	}
}

// Produces HTML showing the (threaded) overview for a group.
func GroupOverview(group string, containers []*Container, out io.Writer) {
	type tmp struct {
		Name     string
		Articles chan template.HTML
	}

	template1 :=
		`<html>
    <head>
        <title>Loread — {{.Name}}</title>
    </head>
    <body>
        <h1>Overview {{.Name}}</h1>
        <ul>
            {{range .Articles}}
                <li>{{.}}</li>
            {{end}}
        </ul>
    </body>
</html>`

	tmpl := template.Must(template.New("initial").Parse(template1))
	ch := make(chan template.HTML, 5)
	go containersToString(ch, containers)

	data := tmp{
		Name:     group,
		Articles: ch,
	}

	err := tmpl.Execute(out, data)

	if err != nil {
		panic(err)
	}
}

// prints cont and its children to ch
func containersToString(ch chan<- template.HTML, cont []*Container) {
	for _, c := range cont {
		walk(ch, c, 0)
	}

	close(ch)
}

// recursive kernel for containersToString
func walk(ch chan<- template.HTML, cont *Container, depth int) {
	ch <- representContainer(cont, depth)

	for c := cont.Child; c != nil; c = c.Next {
		walk(ch, c, depth+1)
	}
}

// prints a container to HTML (which is passed through literally
// by template.Execute)
func representContainer(cont *Container, depth int) template.HTML {
	prefix := ""

	if depth > 0 {
		for i := 0; i < depth; i++ {
			prefix += "  "
		}
	}

	subject := "<<empty container>>"

	v := url.Values{}
	if article := cont.Article; article != nil {
		subject = article.Subject
		v.Set("view", "article")
		v.Set("arg", string(article.Id))
	}

	url := url.URL{
		RawQuery: v.Encode(),
	}

	rv := fmt.Sprintf("<a href=\"%s\">%s %s</a>", url.String(), prefix, subject)
	return template.HTML(rv)
}

// shows cont.Article (where we assume cont.Article != nil)
// nextId is the id of the next article in one of the following
// containers. It needs to be supplied since we can't infer it
// from cont, if there's no article after cont.Article.
func ShowArticle(cont *Container, nextId MessageId, out io.Writer) {
	type tmp struct {
		*Container
		SanitizedText template.HTML
		Next, Back    template.HTML // some links
		HasNext       bool          // is Next set?
	}
	template1 :=
		`<html>
    <head>
        <title>{{.Article.OtherHeaders.From}} — {{.Article.Subject}}</title>
    </head>
    <style>
        .quotation {
            border-left: black thin solid;
            padding-left: .5em
        }
    </style>
    <body>
        <h1>{{.Article.Subject}} <i>{{.Article.OtherHeaders.From}}</i></h1>
<pre>{{.SanitizedText}}</pre>
    </body>
    <table width="100%">
        <tr>
        <td width="20%"><a href="{{.Back}}">Back</a></td>
        {{if .HasNext}}<td align="right" width="80%"><a href="{{.Next}}">Next</a></td>{{end}}
        </tr>
    </table>
</html>`

	tmpl := template.Must(template.New("article").Parse(template1))

	valuesBack := url.Values{}
	valuesBack.Set("delete", string(cont.Article.Id))

	valuesNext := url.Values{}

	// find next article
	var next *Container
	for c := cont; c != nil; c = c.Parent {
		if c.Next != nil && c.Next.Article != nil {
			next = c.Next
			break
		}
	}

	var hasNext bool

	if hasNext := next != nil || nextId != ""; hasNext {
		valuesNext.Set("delete", string(cont.Article.Id))
		valuesNext.Set("view", "article")
		id := ""
		if next == nil {
			id = string(nextId)
		} else {
			id = string(next.Article.Id)
		}
		valuesNext.Set("arg", id)
	}

	urlBack := url.URL{
		RawQuery: valuesBack.Encode(),
	}

	urlNext := url.URL{
		RawQuery: valuesNext.Encode(),
	}

	text := RepresentArticle(*cont.Article)
	data := tmp{cont, text,
		template.HTML(urlNext.String()), template.HTML(urlBack.String()),
		!hasNext}
	err := tmpl.Execute(out, data)

	if err != nil {
		panic(err)
	}
}
