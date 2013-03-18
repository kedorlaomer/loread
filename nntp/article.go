package nntp

import (
	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"unicode"
)

// functions for working with raw and formatted articles

type RawArticle string

type MessageId string

// Parsed article. Its „Body“ still needs formatting, e. g. line
// breaking, recognition of quotations, links, etc. References
// contains message ids. Subject may start with „Re: “ and similar.
// OtherHeaders doesn't contain References etc. Id is a message id.
type FormattedArticle struct {
	References   []MessageId       // collected from References and In-Reply-To headers
	Subject      string            // Subject header
	OtherHeaders map[string]string // remaining headers
	Id           MessageId         // Message ID (as given in the corresponding header)
	Body         string            // unformatted text
}

// Returns all saved articles from „group“.
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

	references := headers["References"]
	inReplyTo := headers["In-Reply-To"]

	if references != "" && inReplyTo != "" {
		first := ""
		// take first that looks like a message id
		for _, ref := range splitByWhite(inReplyTo) {
			if looksLikedMessageId(ref) {
				first = ref
				break
			}
		}

		rawRefs = references + " " + first
	}

	delete(headers, "References")
	delete(headers, "In-Reply-To")
	refs := make([]MessageId, 0)

	for _, ref := range splitByWhite(rawRefs) {
		if ref != "" {
			refs = append(refs, MessageId(TrimWhite(ref)))
		}
	}

	// Subject
	subj := headers["Subject"]
	// base64 or quoted-printable encoded; see RFC 2047
	if len(subj) > 0 && subj[0:2] == "=?" {
		subj = decodeHeader(subj)
	}
	subj = stripPrefixes(subj)
	delete(headers, "Subject")

	// Id
	msgId := headers["Message-Id"]
	delete(headers, "Message-Id")

	/*
	 * encoding/charset issues
	 */

	// Content-Transfer-Encoding
	var err error
	encoding := headers["Content-Transfer-Encoding"]
	var decoded []byte

	switch encoding {
	case "base64":
		decoded, err = base64.StdEncoding.DecodeString(body)

	case "quoted-printable":
		decoded, err = DecodeQuotedPrintable(body)

		// 7bit, 8bit, other unknown types or nil
	default:
		err = nil
		decoded = []byte(body)
	}

	if err != nil {
		panic(fmt.Sprintf("Fehler (?): %s bei Id: %s und Inhalt '%s'\n", err, msgId, body))
	}

	// determine encoding („charset“) from Content-Type
	contentType := headers["Content-Type"]
	contentCharset := "UTF-8" // default charset

	// contentType looks like „text/plain; charset=UTF-8“
	for _, entry := range strings.Split(contentType, ";") {
		entry = TrimWhite(entry)
		if len(entry) > 0 && strings.Index(entry, "charset") >= 0 {
			i := strings.Index(entry, "=")
			if i >= 0 {
				contentCharset = entry[i+1:]

				// maybe the charset is specified with "quotes"
				if contentCharset[0] == '"' {
					contentCharset = contentCharset[1 : len(contentCharset)-1]
				}

				break
			}
		}
	}

	// apply contentCharset
	for {
		// „decoded“ is nil if the Content-Transfer-Encoding was not
		// base64 or quoted-printable
		if normaliseCharset(contentCharset) != "utf8" {
			r, err := charset.NewReader(contentCharset, strings.NewReader(string(decoded)))

			// copy bytes for unknown encoding
			if err != nil {
				body = string(decoded)
				log.Printf("encoding error: %s", err)
				break
			}

			decoded, _ = ioutil.ReadAll(r)
		}

		body = string(decoded)
		break
	}

	return FormattedArticle{
		References:   refs,
		Subject:      subj,
		OtherHeaders: headers,
		Id:           MessageId(msgId),
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
	return strings.Trim(str, "\t\r\n  ")
}

// convert to lower case; remove characters '-', '_', ' '
func normaliseCharset(charset string) string {
	rv := ""
	for _, c := range charset {
		c = unicode.ToLower(c)
		if !(c == '-' || c == '_' || c == ' ') {
			rv += string(c)
		}
	}

	return rv
}

// See RFC 3977, 3.6
func looksLikedMessageId(id string) bool {
	return len(id) > 0 && id[0] == '<' && id[len(id)-1] == '>'
}

// removes prefixes „Re: “, „Aw: “ (we haven't found other
// relevant prefixes)
func stripPrefixes(subj string) string {
	badPrefixes := []string{"re: ", "aw: "}
	redo := true
	for redo {
		redo = false
		for _, prefix := range badPrefixes {
			if len(subj) >= len(prefix) && strings.ToLower(subj[:len(prefix)]) == prefix {
				subj = subj[len(prefix):]
				redo = true
			}
		}
	}

	return subj
}

// see RFC 2047; TODO: duplication of code in FormatArticle?
func decodeHeader(header string) string {
	parts := strings.Split(header, "?")
	contentCharset, encoding, text := parts[1], parts[2], parts[3]
	err := error(nil)
	bytes := []byte{}
	switch strings.ToUpper(encoding) {
	// quoted-printable
	case "Q":
		bytes, err = DecodeQuotedPrintable(text)

	case "B":
		bytes, err = base64.StdEncoding.DecodeString(text)

	default:
		bytes = []byte(fmt.Sprintf("<<Couldn't decode '%s'>>", encoding))
	}

	if err != nil {
		panic(fmt.Sprintf("Fehler (?): %s bei Header: %s\n", header))
	}

	r, err := charset.NewReader(contentCharset, strings.NewReader(string(bytes)))

	if err != nil {
		return "<<Couldn't decode header '" + header + "'>>"
	}

	rv, _ := ioutil.ReadAll(r)
	return string(rv)
}

// splits by white space characters
func splitByWhite(s string) []string {
	canonicizeSpaces := func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}

	s = strings.Map(canonicizeSpaces, s)
	return strings.Split(s, " ")
}
