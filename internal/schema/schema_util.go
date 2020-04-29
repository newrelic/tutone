package schema

import (
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// filterDescription uses a regex to parse certain data out of the
// description of an item
func filterDescription(description string) string {
	var ret string

	re := regexp.MustCompile(`(?s)(.*)\n---\n`)
	desc := re.FindStringSubmatch(description)

	log.Debugf("Description: %#v", desc)

	if len(desc) > 1 {
		ret = desc[1]
	} else {
		ret = description
	}

	return strings.TrimSpace(ret)
}
