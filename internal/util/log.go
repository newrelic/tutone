package util

import (
	log "github.com/sirupsen/logrus"
)

// LogIfError avoids having to constantly nil check error
func LogIfError(lvl log.Level, err error) {
	if err != nil {
		log.StandardLogger().Log(lvl, err)
	}
}
