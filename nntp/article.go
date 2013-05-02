package nntp

import (
	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data" // embed tables into executable
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// functions for working with raw and formatted articles

type RawArticle string

type MessageId string

// Parsed article. Its „Body“ still needs formatting, e. g. line
// breaking, recognition of quotations, links, etc.  Subject may
// start with „Re: “ and similar.  OtherHeaders doesn't contain
// References etc.
type ParsedArticle struct {
	References   []MessageId       // collected from References and In-Reply-To headers
	Subject      string            // Subject line
	Id           MessageId         // Message ID (as given in the corresponding header)
	OtherHeaders map[string]string // remaining headers
	Body         string            // unformatted text, converted to UTF-8
	Date         time.Time         // Date header (already parsed)
}

// Returns all saved articles from „group“ and the paths of
// their files.
func GetArticles(group string) ([]RawArticle, []string, error) {
	info, err := ioutil.ReadDir(group)

	if err != nil {
		return nil, nil, err
	}

	rv := make([]RawArticle, 0)
	paths := make([]string, 0)
	for _, fileInfo := range info {
		name := fileInfo.Name()
		if name[0] != '.' { // ignore .watermark
			name = group + "/" + fileInfo.Name()
			data, err := ioutil.ReadFile(name)

			if err != nil {
				return nil, nil, err
			}

			rv = append(rv, RawArticle(data))
			paths = append(paths, name)
		}
	}

	return rv, paths, err
}

// Separates body and headers; determines subject, references
// etc.; deals with encoding and charset issues.
func FormatArticle(article RawArticle) ParsedArticle {
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
		for _, ref := range SplitByWhite(inReplyTo) {
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

	for _, ref := range SplitByWhite(rawRefs) {
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

	var aTime time.Time
	if date, ok := headers["Date"]; ok {
		// we found all these date formats in our corpus,
		// containing 40000+ messages from comp.lang.forth
		// comp.lang.lisp, comp.lang.haskell and
		// rec.games.abstract
		layouts := []string{
			"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
			"Mon, 2 Jan 2006 15:04:05 -0700",
			"Mon, 2 Jan 2006 15:04:05 MST",
			"Mon, 2 Jan 2006 15:04:05 -0700 (MST-07:00)",
			"2 Jan 2006 15:04:05 -0700",
			"2 Jan 2006 15:04:05 MST",
			"Mon, 2 Jan 2006 15:04 -0700",
		}

		for _, layout := range layouts {
			aTime, err = time.Parse(layout, date)
			if err == nil {
				break
			}
		}
	}

	return ParsedArticle{
		References:   refs,
		Subject:      subj,
		OtherHeaders: headers,
		Id:           MessageId(msgId),
		Body:         body,
		Date:         aTime,
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
	return strings.TrimFunc(str, unicode.IsSpace)
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
func SplitByWhite(s string) []string {
	canonicizeSpaces := func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}

	s = strings.Map(canonicizeSpaces, s)
	return strings.Split(s, " ")
}
