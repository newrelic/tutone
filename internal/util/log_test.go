//go:build unit
// +build unit

package util

import (
	"errors"
	"io"
	"testing"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestLogIfError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level log.Level
		err   error
	}{
		{log.PanicLevel, nil}, // Should not panic
		{log.WarnLevel, errors.New("this should be a warn log")},
		{log.InfoLevel, errors.New("this should be an info log")},
		{log.ErrorLevel, nil}, // Should not error
	}

	// LogIfError uses log.StandardLogger() so we have to override some of it, and add a hook
	// to catch the messages
	stdLogger := log.StandardLogger()
	hook := logtest.NewGlobal()

	oldOut := stdLogger.Out
	stdLogger.Out = io.Discard
	defer func() { stdLogger.Out = oldOut }()

	for x := range cases {
		LogIfError(cases[x].level, cases[x].err)

		if cases[x].err != nil {
			assert.Equal(t, cases[x].err.Error(), hook.LastEntry().Message)
			hook.Reset()
		} else {
			assert.Nil(t, hook.LastEntry())
		}
	}
}
