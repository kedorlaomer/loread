package nntp

import (
	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// functions for working with raw and formatted articles
type RawArticle string

// Parsed article. Its „Text“ still needs formatting, e. g. line
// breaking, recognition of quotations, links, etc.
type FormattedArticle struct {
	References   []string          // collected from References and In-Reply-To headers
	Subject      string            // Subject header; has „Re: “ and similar removed
	OtherHeaders map[string]string // remaining headers
	Id           string            // Message ID (as given in the corresponding header)
	Body         string            // unformatted text
}

// Returns all articles from „group“.
func GetArticles(group string) ([]RawArticle, error) {
	info, err := ioutil.ReadDir(group)

	if err != nil {
		return nil, err
	}

	rv := make([]RawArticle, 0)
	for _, fileInfo := range info {
		name := fileInfo.Name()
		if name[0] != '.' {
			name = group + "/" + fileInfo.Name()
			data, err := ioutil.ReadFile(name)

			if err != nil {
				return nil, err
			}

			rv = append(rv, RawArticle(data))
		}
	}

	return rv, err
}

// Separates body and headers; determines subject, references
// etc.; deals with encoding and charset issues.
func FormatArticle(article RawArticle) FormattedArticle {
	rawHeaders, body := firstAndRest(string(article), "\n\n")
	body = TrimWhite(body)

	// every element is one header line
	joinedHeaders := make([]string, 0)

	buf := ""

	// some headers are multiline (see RFC 3977, 3.6, „folded“)
	for _, line := range strings.Split(rawHeaders, "\n") {
		firstChar := line[0]
		// line for itself
		if firstChar != '\t' && firstChar != ' ' && len(buf) > 0 {
			joinedHeaders = append(joinedHeaders, TrimWhite(buf))
			buf = ""
		}

		buf = buf + line + "\n"
	}

	// all headers
	headers := make(map[string]string)

	for _, headerLine := range joinedHeaders {
		key, value := firstAndRest(headerLine, ": ")
		key = http.CanonicalHeaderKey(key)
		headers[key] = value
	}

	/*
	 * some important headers
	 */

	// References, In-Reply-To
	rawRefs := headers["References"] + " " + headers["In-Reply-To"]
	delete(headers, "References")
	delete(headers, "In-Reply-To")
	refs := make([]string, 0)

	for _, ref := range strings.Split(rawRefs, " ") {
		if ref != "" {
			refs = append(refs, ref)
		}
	}

	// Subject
	subj := headers["Subject"]
	delete(headers, "Subject")

	// Id
	msgId := headers["Message-Id"]
	delete(headers, "Message-Id")

	// Content-Transfer-Encoding
	var err error
	encoding := headers["Content-Transfer-Encoding"]
	var decoded []byte

	switch encoding {
	case "base64":
		decoded, err = base64.StdEncoding.DecodeString(body)

	case "quoted-printable":
		decoded, err = DecodeQuotedPrintable(body)

		// 7bit, 8bit, other unknown types
	default:
		err = nil
	}

	if err != nil {
		panic(fmt.Sprintf("Fehler (?): %s bei Id: %s und Inhalt '%s'\n", err, msgId, body))
	}

	// determine encoding („charset“) from Content-Type
	contentType := headers["Content-Type"]
	contentCharset := "UTF-8" // default charset

	// contentType looks like „text/plain; charset=UTF-8“
	for _, entry := range strings.Split(contentType, "; ") {
		if len(entry) > 0 && strings.Index(entry, "charset") >= 0 {
			i := strings.Index(entry, "=")
			if i >= 0 {
				contentCharset = entry[i+1:]
				break
			}
		}
	}

    // apply contentCharset
	for {
		// „decoded“ is nil if the Content-Transfer-Encoding was not
		// base64 or quoted-printable
		if decoded != nil {
			r, err := charset.NewReader(contentCharset, strings.NewReader(string(decoded)))

			// copy bytes for unknown encoding
			if err != nil {
				body = string(decoded)
				break
			}

			decoded, _ = ioutil.ReadAll(r)
			body = string(decoded)
		}
		break
	}

	return FormattedArticle{
		References:   refs,
		Subject:      subj,
		OtherHeaders: headers,
		Id:           msgId,
		Body:         body,
	}
}

// example: firstAndRest("this: is: an example", ": ") → "this",
// "is: an example"
func firstAndRest(str, sep string) (first, rest string) {
	parts := strings.Split(str, sep)
	first = parts[0]
	if len(parts) == 0 {
		rest = ""
	} else {
		rest = strings.Join(parts[1:], sep)
	}

	return
}

func TrimWhite(str string) string {
	return strings.Trim(str, "\t\r\n ")
}
