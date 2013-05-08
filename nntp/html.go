package nntp

import (
	"fmt"
	"html/template"
	"io"
	"net/url"
)

// Shows a good bye screen.
func FinalScreen(out io.Writer) {
	text :=
		`<html>
    <head>
        <title>Loread — The low reader</title>
    </head>
    <body>
        <h1>Good bye…</h1>
    </body>
</html>`
	out.Write([]byte(text))
}

// Produces HTML for an initial screen listing all subscribed
// groups.
func InitialScreen(groups []string, out io.Writer) {
	template1 :=
		`<html>
    <head>
        <title>Loread — The low reader</title>
    </head>
    <body>
        <h1>Your subscribed groups</h1>
        <ul>
            {{range .}}
                <li><a href="?arg={{.}}&view=group">{{.}}</a></li>
            {{else}}
                Nothing?
            {{end}}
        </ul>
        <a href="?view=quit">Quit</a>
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
		Back     string
	}

	template1 :=
		`<html>
    <head>
        <title>Loread — {{.Name}}</title>
    </head>
    <body>
        <a href="{{.Back}}">Back</a>
        <h1>Overview {{.Name}}</h1>
        <ul>
            {{range .Articles}}
                <li>{{.}}</li>
            {{end}}
        </ul>
        <a href="{{.Back}}">Back</a>
    </body>
</html>`

	tmpl := template.Must(template.New("initial").Parse(template1))
	ch := make(chan template.HTML, 5)
	go containersToString(ch, containers)

	backUrl := url.URL{
		RawQuery: url.Values{
			"view": {"overview"},
		}.Encode()}

	data := tmp{
		Name:     group,
		Articles: ch,
		Back:     backUrl.String(),
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
	if cont.Article != nil {
		ch <- representContainer(cont, depth)

		for c := cont.Child; c != nil; c = c.Next {
			walk(ch, c, depth+1)
		}
	}
}

// Prints a container to HTML (which is passed through literally
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

// Shows cont.Article (where we assume cont.Article != nil).
// Since it's not possible to find out from the container which
// group it belongs to (it could have several groups listed in
// cont.Article.OtherHeaders["Newsgroups"]), we need provide
// this information.
func ShowArticle(cont *Container, fromGroup string, out io.Writer) {
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
        <table width="100%">
            <tr>
                <td width="20%">
                    <a href="{{.Back}}">Back</a>
                </td>
                {{if .HasNext}}<td align="right" width="80%"><a href="{{.Next}}">Next</a></td>{{end}}
            </tr>
        </table>
        <h1>{{.Article.Subject}} <i>{{.Article.OtherHeaders.From}}</i></h1>
<pre>{{.SanitizedText}}</pre>
        <table width="100%">
            <tr>
                <td width="20%">
                    <a href="{{.Back}}">Back</a>
                </td>
                {{if .HasNext}}<td align="right" width="80%"><a href="{{.Next}}">Next</a></td>{{end}}
            </tr>
        </table>
    </body>
</html>`

	tmpl := template.Must(template.New("article").Parse(template1))

	valuesBack := url.Values{}
	valuesBack.Set("delete", string(cont.Article.Id))
	valuesBack.Set("view", "group")
	valuesBack.Set("arg", fromGroup)

	valuesNext := url.Values{}

	// find next article
	var next *Container

	if cont.Child != nil {
		next = cont.Child
	} else {
		for c := cont; c != nil; c = c.Parent {
			if c.Next != nil && c.Next.Article != nil {
				next = c.Next
				break
			}
		}
	}

	if next != nil {
		valuesNext.Set("delete", string(cont.Article.Id))
		valuesNext.Set("view", "article")
		if next != nil {
			valuesNext.Set("arg", string(next.Article.Id))
		}
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
		next != nil}
	err := tmpl.Execute(out, data)

	if err != nil {
		panic(err)
	}
}

// Displays an error page showing data (as formatted via
// fmt.Sprintf's %+v control)
func ErrorPage(err interface{}, out io.Writer) {
	type tmp struct {
		Error string
	}
	template1 :=
		`<html>
    <head>
        <title>Error</title>
    </head>
    <body>
        <h1>Error</h1>
        An error occurred: {{.Error}}
        <a href="?view=overview">Main Screen</a>
    </body>
</html>`

	data := tmp{
		Error: fmt.Sprintf("%+v", err),
	}

	tmpl := template.Must(template.New("error").Parse(template1))
	err2 := tmpl.Execute(out, data)

	if err2 != nil {
		panic(err)
	}
}

// Similar to ErrorPage, but uses a fmt.Sprintf format. Due to
// this, out can't be supplied last as in ErrorPage.
func ErrorPageF(out io.Writer, format string, other ...interface{}) {
	text := fmt.Sprintf(format, other...)
	ErrorPage(text, out)
}
