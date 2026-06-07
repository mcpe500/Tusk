package main

import (
	"bufio"
	"bytes"
)

func readJSONObject(reader *bufio.Reader) ([]byte, error) {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == '{' {
			if err := reader.UnreadByte(); err != nil {
				return nil, err
			}
			break
		}
	}

	var buf bytes.Buffer
	braces := 0
	inString := false
	escaped := false

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		buf.WriteByte(b)

		if escaped {
			escaped = false
			continue
		}
		if b == '\\' {
			escaped = true
			continue
		}
		if b == '"' {
			inString = !inString
			continue
		}
		if !inString {
			if b == '{' {
				braces++
			} else if b == '}' {
				braces--
				if braces == 0 {
					return buf.Bytes(), nil
				}
			}
		}
	}
}
