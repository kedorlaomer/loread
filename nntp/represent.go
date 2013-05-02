package nntp

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"unicode"
)

type indentedLine struct {
	line  string // line without leading quotation marks
	depth int    // number of leading quotation marks
}

// a group of (ideally) indentedLine with equal depth
type block []indentedLine

// similar to representContainer
func RepresentArticle(article ParsedArticle) template.HTML {
	text := article.Body

	// TODO:
	// * signatures
	// * remove empty lines inserted by Google Groups

	// assign a depth to each line
	lines := strings.Split(text, "\n")
	indented := make([]indentedLine, len(lines))

	for i, line := range lines {
		indented[i] = indentedLine{
			line:  stripQuotation(line),
			depth: depth(line),
		}
	}

	// group lines of equal length
	blocks := make([]block, 0)
	lastBlock := make(block, 0)
	lastDepth := 0

	for _, line := range indented {
		// blank lines separate groups
		if line.depth != lastDepth || len(TrimWhite(line.line)) == 0 {
			blocks = append(blocks, lastBlock)
			lastBlock = make(block, 0)
			lastDepth = line.depth
		}

		lastBlock = append(lastBlock, line)
	}

	// don't forget last block
	blocks = append(blocks, lastBlock)

	// Determine if a block needs to be reflowed. We use the
	// following strategy: If a line is longer than MAX_LENGTH,
	// the whole block will be reflowed to OPTIMUM_LENGTH.
	const (
		MAX_LENGTH     = 96 // accept somewhat long lines
		OPTIMUM_LENGTH = 64 // but reflow aggressively
	)

	// reflow
	for i, block := range blocks {
		if len(block) > 0 {
			for _, indented := range block {
				if len(indented.line) > MAX_LENGTH { // needs reflow
					blocks[i] = reflow(block, OPTIMUM_LENGTH)
				}
			}
		}
	}

	// Add <div class="quotation">…</div> around blocks. It's
	// important that they be always at the beginning of a line.

	lastDepth = 0
	rv := new(bytes.Buffer)

	for _, block := range blocks {
		if len(block) > 0 {
			currentDepth := block[0].depth

			if lastDepth < currentDepth { // indent
				for i := lastDepth; i < currentDepth; i++ {
					fmt.Fprint(rv, "<div class=\"quotation\">")
				}
			} else if lastDepth > currentDepth { // unindent
				for i := currentDepth; i < lastDepth; i++ {
					fmt.Fprint(rv, "</div>")
				}
			}

			// TODO: recognize emoticons, links in indented.line
			for _, indented := range block {
				fmt.Fprintln(rv, template.HTMLEscapeString(indented.line))
			}

			lastDepth = currentDepth
		}
	}

	return template.HTML(rv.Bytes())
}

// How deeply is line quoted? I. e., how many leading quotation
// marks ">" does line have?
//
// There are alternative posting styles, e. g., leading "|" or
// abbreviating the original poster's name, as in "nn>". These
// are rarely used in our corpus, so we ignore them.
func depth(line string) int {
	rv := 0

	for _, c := range line {
		if c == '>' {
			rv++
		} else if !unicode.IsSpace(c) {
			return rv
		}
	}

	return rv
}

// Strip leading ">" and whitespace.
func stripQuotation(line string) string {
	for i, c := range line {
		if !unicode.IsSpace(c) && c != '>' {
			// found first content character; search backwards
			// for beginning of the string or last quotation
			// mark
			for j := i; j >= 0; j-- {
				if line[j] == '>' {
					return line[j+1:]
				}
			}

			return line
		}
	}

	return ""
}

func reflow(b block, length int) block {
	rv := make(block, 0)
	buffer := ""
	depth := b[0].depth

	for _, indented := range b {
		for _, word := range SplitByWhite(indented.line) {
			if len(buffer)+1+len(word) > length {
				rv = append(rv, indentedLine{buffer, depth})
				buffer = word
			} else {
				buffer += " " + word
			}
		}
	}

	// don't forget last line
	if len(buffer) > 0 {
		rv = append(rv, indentedLine{buffer, depth})
	}

	return rv
}
