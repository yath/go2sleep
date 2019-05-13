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

type handlerFunc func(context.Context, *device, []string) error

// tui is a text user interface.
type tui struct {
	rl *readline.Instance
	h  map[string]handlerFunc

	d *device
}

// setLogWriter sets the global logging backend to w and enables colors.
func setLogWriter(w io.Writer) {
	b := logging.NewLogBackend(w, "", golog.LstdFlags)
	b.Color = true
	logging.SetBackend(b)
}

// newTUI sets up readline, redirects logging and returns a new *tui instance.
func newTUI(h map[string]handlerFunc, d *device) (*tui, error) {
	rl, err := readline.New("Command> ")
	if err != nil {
		return nil, fmt.Errorf("can't get a readline instance: %v", err)
	}

	setLogWriter(rl)

	return &tui{rl, h, d}, nil
}

// close resets logging to its default values and closes readline.
func (t *tui) close() error {
	log.Debugf("Closing UI.")
	logging.Reset()
	return t.rl.Close()
}

// loop processes commands retrieved from readline until the context expires.
func (t *tui) loop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
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
				log.Errorf("Unknown command %q.", cmd)
				continue
			}

			if err := h(ctx, t.d, f[1:]); err != nil {
				if err == io.EOF {
					return err
				}
				log.Errorf("Error processing command %q: %v.", cmd, err)
			}
		}
	}
}
