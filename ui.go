package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	golog "log"

	"github.com/chzyer/readline"
	"github.com/op/go-logging"
)

type handlerFunc func([]string) error

type tui struct {
	rl *readline.Instance
	h  map[string]handlerFunc
}

func setLogWriter(w io.Writer) {
	b := logging.NewLogBackend(w, "", golog.LstdFlags)
	b.Color = true
	logging.SetBackend(b)
}

func newTUI(h map[string]handlerFunc) (*tui, error) {
	rl, err := readline.New("Command> ")
	if err != nil {
		return nil, fmt.Errorf("can't get a readline instance: %v", err)
	}

	setLogWriter(rl)

	return &tui{rl, h}, nil
}

func (t *tui) close() error {
	log.Debugf("Closing UI")
	logging.Reset()
	return t.rl.Close()
}

func (t *tui) loop(_ context.Context) error {
	for {
		line, err := t.rl.Readline()
		if err != nil {
			return err
		}

		f := strings.Fields(line)
		if len(f) < 1 || f[0] == "" {
			continue
		}

		cmd := f[0]
		h, ok := t.h[cmd]
		if !ok {
			log.Errorf("Unknown command %q", cmd)
			continue
		}

		if err := h(f); err != nil {
			if err == io.EOF {
				return err
			}
			log.Errorf("Error processing command %q: %v", cmd, err)
		}
	}
}
