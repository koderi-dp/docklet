package compose

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"unicode/utf8"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// TextStyle is what the editor must restore when it saves: the file's line ending and BOM.
type TextStyle struct{ CRLF, BOM bool }

// SplitText turns file bytes into editor text (LF only, no BOM) and the style to restore.
// A file with more CRLF than bare LF counts as CRLF.
func SplitText(data []byte) (string, TextStyle, error) {
	var st TextStyle
	if bytes.HasPrefix(data, utf8BOM) {
		st.BOM = true
		data = data[len(utf8BOM):]
	}
	if !utf8.Valid(data) {
		return "", st, errors.New("the file is not valid UTF-8 text")
	}
	crlf := bytes.Count(data, []byte("\r\n"))
	lf := bytes.Count(data, []byte("\n")) - crlf
	st.CRLF = crlf > lf
	return string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), st, nil
}

// JoinText is the inverse of SplitText.
func JoinText(text string, s TextStyle) []byte {
	b := []byte(text)
	if s.CRLF {
		b = bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
	}
	if s.BOM {
		b = append(append([]byte{}, utf8BOM...), b...)
	}
	return b
}

// FileVersion identifies the exact bytes of a file.
func FileVersion(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
