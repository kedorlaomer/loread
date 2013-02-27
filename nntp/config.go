// reading configuration files
//
// preferred format:
//
// key: value

package nntp

import (
	"bufio"
	"io"
	"log"
	"os"
	"strings"
)

func ReadConfig(filename string) (config map[string]string, err error) {
	const SEP = ": "
	const LEN = len(SEP)
	config = make(map[string]string, 50)
	file, err := os.Open(filename)

	if err != nil {
		return
	}

	defer file.Close()
	lineReader := bufio.NewReader(file)

	for line, err2 := lineReader.ReadString('\n'); line != ""; line, err2 = lineReader.ReadString('\n') {
		i := strings.Index(line, SEP)

		if i == -1 {
			log.Printf("config.Read: error in config file %s\n", filename)
			continue
		}

		key, value := line[:i], line[i+LEN:]
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		config[key] = value

		if err2 == io.EOF {
			err = nil
			return
		}

		if err2 != nil {
			err = err2
			return
		}
	}

	return
}
