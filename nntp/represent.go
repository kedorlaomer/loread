package nntp

import (
	"html/template"
)

// similar to representContainer
func RepresentArticle(article ParsedArticle) template.HTML {
	text := template.HTMLEscapeString(article.Body)
	// TODO:
	// * break long lines (very necessary for quoted-printable!)
	// * deal with quotations
	// * emoticons (replace by unicode?)
	// * signatures
	// * recognize links
	return template.HTML(text)
}
