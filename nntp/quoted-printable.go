package nntp

import (
	"encoding/hex"
)

// See RFC 2045
func DecodeQuotedPrintable(str string) ([]byte, error) {
	var rv []byte = make([]byte, 0)

	// we manually change i, so no range construct
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c != '=' {
			// literal
			rv = append(rv, c)
		} else {
			// soft break or hex-quoted
			next := str[i+1]
			// CRLF pairs are already translated into single
			// '\n', so only proceed one character
			if next == '\t' || next == '\n' {
				i++
			} else {
				// hex-quoted → grab two hex digits
				bytes, err := hex.DecodeString(str[i+1 : i+3])
				if err != nil {
					return rv, err
				} else {
					// „bytes“ should be exactly one byte
					rv = append(rv, bytes[0])
				}

				i = i + 2
			}
		}
	}

	return rv, nil
}
