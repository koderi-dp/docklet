package services

import (
	"iter"

	"github.com/moby/moby/api/types/jsonstream"
)

type sink struct {
	logs     []string
	progress []float64
	detail   string
}

func (s *sink) Log(l string) { s.logs = append(s.logs, l) }

func (s *sink) Detail(d string) { s.detail = d }

func (s *sink) Progress(p float64) { s.progress = append(s.progress, p) }

func seq(msgs []jsonstream.Message) iter.Seq2[jsonstream.Message, error] {
	return func(yield func(jsonstream.Message, error) bool) {
		for _, m := range msgs {
			if !yield(m, nil) {
				return
			}
		}
	}
}

func dl(id, status string, cur, tot int64) jsonstream.Message {
	m := jsonstream.Message{ID: id, Status: status}
	if tot > 0 {
		m.Progress = &jsonstream.Progress{Current: cur, Total: tot}
	}
	return m
}
