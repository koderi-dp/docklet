package registryauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// configDoc is an editable view of config.json that keeps every key, its
// order and its raw value, so fields Docklet does not know survive a rewrite.
type configDoc struct {
	keys   []string
	vals   map[string]json.RawMessage
	indent string
	noNL   bool // the original file had no trailing newline
}

func parseConfigDoc(b []byte) (*configDoc, error) {
	d := &configDoc{vals: map[string]json.RawMessage{}, indent: detectIndent(b), noNL: len(b) > 0 && !bytes.HasSuffix(b, []byte{'\n'})}
	if len(bytes.TrimSpace(b)) == 0 {
		return d, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New("Docker config.json is not a JSON object")
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, errors.New("Docker config.json is not valid JSON")
		}
		key, _ := kt.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, errors.New("Docker config.json is not valid JSON")
		}
		if _, dup := d.vals[key]; !dup {
			d.keys = append(d.keys, key)
		}
		d.vals[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, errors.New("Docker config.json is not valid JSON")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("Docker config.json is not valid JSON")
	}
	return d, nil
}

func detectIndent(b []byte) string {
	i := bytes.IndexByte(b, '\n')
	if i < 0 {
		return "\t"
	}
	rest := b[i+1:]
	n := 0
	for n < len(rest) && (rest[n] == ' ' || rest[n] == '\t') {
		n++
	}
	if n == 0 {
		return "\t"
	}
	return string(rest[:n])
}

// auths returns the "auths" object as an ordered key list with raw values.
func (d *configDoc) auths() (*configDoc, error) {
	raw, ok := d.vals["auths"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return &configDoc{vals: map[string]json.RawMessage{}}, nil
	}
	a, err := parseConfigDoc(raw)
	if err != nil {
		return nil, errors.New("the auths section of Docker config.json is not an object")
	}
	return a, nil
}

func (d *configDoc) set(key string, raw json.RawMessage) {
	if _, ok := d.vals[key]; !ok {
		d.keys = append(d.keys, key)
	}
	d.vals[key] = raw
}

func (d *configDoc) remove(key string) bool {
	if _, ok := d.vals[key]; !ok {
		return false
	}
	delete(d.vals, key)
	for i, k := range d.keys {
		if k == key {
			d.keys = append(d.keys[:i], d.keys[i+1:]...)
			break
		}
	}
	return true
}

func (d *configDoc) encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range d.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := marshalNoEscape(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(d.vals[k])
	}
	buf.WriteByte('}')
	var out bytes.Buffer
	if err := json.Indent(&out, buf.Bytes(), "", d.indent); err != nil {
		return nil, err
	}
	if !d.noNL {
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// editAuths applies fn to the auths object of the config file at path and
// writes the result atomically. Nothing is written when the file cannot be
// parsed or fn changes nothing. A missing file is created only when create is true.
func editAuths(path string, create bool, fn func(a *configDoc) bool) error {
	if path == "" {
		return errors.New("cannot locate the Docker config directory")
	}
	b, err := os.ReadFile(path)
	mode := os.FileMode(0o600)
	switch {
	case os.IsNotExist(err):
		if !create {
			return nil
		}
		b = nil
	case err != nil:
		return errors.New("cannot read the Docker config file")
	default:
		if st, serr := os.Stat(path); serr == nil {
			mode = st.Mode().Perm()
		}
	}
	doc, err := parseConfigDoc(b)
	if err != nil {
		return err
	}
	auths, err := doc.auths()
	if err != nil {
		return err
	}
	if !fn(auths) {
		return nil
	}
	rawAuths, err := auths.encodeCompactObject()
	if err != nil {
		return err
	}
	doc.set("auths", rawAuths)
	out, err := doc.encode()
	if err != nil {
		return err
	}
	return writeFileAtomic(path, out, mode)
}

func (d *configDoc) encodeCompactObject() (json.RawMessage, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range d.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := marshalNoEscape(k)
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(d.vals[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create the Docker config directory")
	}
	tmp, err := os.CreateTemp(dir, ".config.json.docklet-*")
	if err != nil {
		return errors.New("cannot write the Docker config file")
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(name)
		return errors.New("cannot write the Docker config file")
	}
	_ = os.Chmod(name, mode)
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return errors.New("cannot replace the Docker config file")
	}
	return nil
}

// markHelperEntry mirrors docker login with a credential helper: the helper
// holds the secret and config.json gets an empty auths entry for the server.
func markHelperEntry(path, serverKey string) error {
	return editAuths(path, false, func(a *configDoc) bool {
		if _, ok := a.vals[serverKey]; ok {
			return false // an existing entry (even with inline data) is left alone
		}
		a.set(serverKey, json.RawMessage("{}"))
		return true
	})
}

// removeAuthsEntry deletes the auths entry whose key normalizes to host. With
// onlyEmpty, entries that carry data are kept (they are not helper markers).
func removeAuthsEntry(path, host string, onlyEmpty bool) error {
	return editAuths(path, false, func(a *configDoc) bool {
		changed := false
		for _, k := range append([]string(nil), a.keys...) {
			if NormalizeHost(k) == host && (!onlyEmpty || string(bytes.TrimSpace(a.vals[k])) == "{}") {
				a.remove(k)
				changed = true
			}
		}
		return changed
	})
}
