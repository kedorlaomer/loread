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

// similar to representContainer
func representArticle(article ParsedArticle) template.HTML {
	text := template.HTMLEscapeString(article.Body)
	// TODO:
	// * deal with quotations
	// * recognize links
	// * break long lines (very necessary for quoted-printable!)
	return template.HTML(text)
}

// shows cont.Article (where we assume cont.Article != nil)
func ShowArticle(cont *Container, out io.Writer) {
	type tmp struct {
		*Container
		SanitizedText template.HTML
        // TODO: links up, next, parent, back to group…
	}
	template1 :=
		`<html>
    <head>
        <title>{{.Article.OtherHeaders.From}} — {{.Article.Subject}}</title>
    </head>
    <body>
        <h1>{{.Article.Subject}}</h1>
        {{.SanitizedText}}
    </body>
    <table>
        <tr>
        <td>{{}}</td>
        </tr>
    </table>
</html>`

	tmpl := template.Must(template.New("article").Parse(template1))
	data := tmp{cont, representArticle(*cont.Article)}

	err := tmpl.Execute(out, data)

	if err != nil {
		panic(err)
	}
}
