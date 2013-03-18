package nntp

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/textproto"
	"os"
	"strconv"
	"strings"
)

// NNTP protocol codes (see RFC 3977; https://tools.ietf.org/html/rfc3977)
const HELLO = 200
const PASSWORD_REQUIRED = 381
const AUTHEN_ACCEPTED = 281
const PERM_MASK = 0777

// generic OK
const OK = 211

// ANSI color codes for coloring input and output
const (
	CODE_INPUT  = "\x1B[32m" // green font
	CODE_OUTPUT = "\x1B[34m" // blue font
	CODE_RESET  = "\x1B[39m" // reset
)

var verbose bool

type Conn struct {
	intern *textproto.Conn
}

// Fetches articles as specified in the configuration.
func FetchArticles(config map[string]string) {
	network := "tcp"
	server, port := config["server"], config["port"]
	username, passw := config["login"], config["pass"]
	fetchMaximum := atoi(config["fetch-maximum"], 100) // reasonable (?) default
	_, verbose = config["verbose"]

	if server == "" || port == "" {
		log.Fatal("Port or server not given. Config says: server = '%s', port = '%s'\n",
			server, port)
	}

	if username == "" {
		log.Fatal("Username not given.\n")
	}

	addr := config["server"] + ":" + config["port"]

	// connect
	conn1, err := textproto.Dial(network, addr)
	die(err, "Couldn't dial %s (error: %s)", network, err)
	conn := Conn{conn1}

	defer conn.Close()

	// say hello
	code, message, err := conn.ReadCodeLine(HELLO)
	die(err, "Couldn't connect to server. Error %d, message %s.\n", code, message)
	_, err = conn.Cmd("AUTHINFO USER %s", username)
	die(err, "Didn't like AUTHINFO USER (%s)\n", err)

	// authenticate
	code, message, err = conn.ReadCodeLine(PASSWORD_REQUIRED)
	if code != PASSWORD_REQUIRED && code != AUTHEN_ACCEPTED {
		die(err, "unexpected code: %d (%s) (%s)\n", code, message, err)
	}

	if code == PASSWORD_REQUIRED {
		_, err = conn.Cmd("AUTHINFO PASS %s", passw)
		die(err, "Didn't like AUTHINFO PASSW (%s)\n", err)
		code, message, err = conn.ReadCodeLine(AUTHEN_ACCEPTED)
		die(err, "Didn't like AUTHINFO PASSW (%s) (%s)\n", message, err)
	}

	groups := strings.Split(config["groups"], ", ")
	if len(groups) == 0 {
		log.Fatal("No groups given.\n")
	}

	// fetch articles
	for _, g := range groups {
		err = os.Mkdir(g, PERM_MASK) // everyone may read/write this
		if err != nil && !os.IsExist(err) {
			die(err, "Couldn't create directory %s (%s)", g, err.Error())
		}

		// select group; get server's watermark
		_, err = conn.Cmd("GROUP %s", g)
		die(err, "Couldn't choose group %s", g)
		code, message, err = conn.ReadCodeLine(OK)
		die(err, "Couldn't choose group %s", g)

		parts := strings.Split(message, " ")
		if len(parts) != 4 {
			log.Fatal("Expected four parts, but got '%s' with %d parts", message, len(parts))
		}

		number := atoi(parts[0], -1)
		lo, hi := atoi(parts[1], -1), atoi(parts[2], -1)

		if number < 0 || lo < 0 || hi < 0 {
			log.Fatal("Server answered: %d, %d, %d", number, lo, hi)
		}

		watermark := GetWatermark(g)

		// we can't catch up with server anymore because we are
		// too far behind
		if watermark < lo {
			watermark = lo
		}

		// get a list of article numbers
		_, err = conn.Cmd("LISTGROUP %s %d-", g, watermark+1)
		die(err, "Couldn't list group %s", g)
		articles, err := conn.ReadDotLines()
		die(err, "Couldn't list group %s", g)
		articles = articles[1:]

		// get only the last fetchMaximum articles
		if len(articles) > fetchMaximum {
			articles = articles[len(articles)-fetchMaximum:]
		}

		// number of last read article
		lastRead := watermark

		// save articles
		for _, no := range articles {
			err = fetchArticle(conn, g, no)
			lastRead = atoi(no, lastRead)
			if err != nil {
				break
			}
		}

		if len(articles) == 0 {
			lastRead = hi
		} else {
			lastRead = atoi(articles[len(articles)-1], hi)
		}

		if watermark > lastRead {
			lastRead = watermark
		}

		SetWatermark(g, lastRead)
	}

	// is allowed to fail
	conn.Cmd("QUIT")
}

func die(err error, format string, args ...interface{}) {
	if err != nil {
		log.Fatal(format, args)
	}
}

// Converts str into an int. Returns n if str is malformed.
func atoi(str string, n int) int {
	rv, err := strconv.Atoi(str)
	if err != nil {
		return n
	}

	return rv
}

// Reads the file „groupname“/.watermark which should contain a number. This
// should be the last message read. Returns this number (or 0,
// if there's no such number).
func GetWatermark(groupname string) int {
	name := groupname + "/.watermark"

	everything, err := ioutil.ReadFile(name)

	if err != nil {
		return 0
	}

	number := strings.TrimSpace(string(everything))
	return atoi(number, 0)
}

// See GetWatermark.
func SetWatermark(groupname string, messageNo int) error {
	filename := groupname + "/.watermark"
	data := []byte(strconv.Itoa(messageNo))
	return ioutil.WriteFile(filename, data, PERM_MASK)
}

// Saves article „messageNo“ from „groupname“ that has the text
// „content“. Should contain both header and article text.
func WriteArticle(groupname string, messageNo string, content string) error {
	filename := groupname + "/" + messageNo
	data := []byte(content)
	return ioutil.WriteFile(filename, data, PERM_MASK)
}

// Like fmt.Printf, but only if verbose was set in the config
// file.
func printVerbosely(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format, args...)
	}
}

// We wrap textproto.Conn's functions with verbose (and
// colorful) printing, if „verbose“ is set in the config file.
func (conn Conn) Cmd(format string, args ...interface{}) (id uint, err error) {
	printVerbosely(CODE_OUTPUT)
	defer printVerbosely(CODE_RESET)
	printVerbosely(format+"\n", args...)
	id, err = conn.intern.Cmd(format, args...)
	return
}

func (conn Conn) Close() error {
	return conn.intern.Close()
}

func (conn Conn) ReadCodeLine(expected int) (code int, message string, err error) {
	code, message, err = conn.intern.ReadCodeLine(expected)
	printVerbosely(CODE_INPUT)
	defer printVerbosely(CODE_RESET)
	printVerbosely("(code %d) %s\n", code, message)
	return
}

func (conn Conn) ReadDotLines() (lines []string, err error) {
	lines, err = conn.intern.ReadDotLines()
	printVerbosely(CODE_INPUT)
	defer printVerbosely(CODE_RESET)
	if len(lines) > 0 {
		for _, line := range lines {
			printVerbosely("%s\n", line)
		}
	}
	return
}

func fetchArticle(conn Conn, group string, no string) error {
	_, err := conn.Cmd("ARTICLE %s", no)

	if err != nil {
		return err
	}

	lines, err := conn.ReadDotLines()
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	article := strings.Join(lines[1:], "\n") // first line is error code etc.
	return WriteArticle(group, no, article)
}
