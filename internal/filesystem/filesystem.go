package filesystem

import (
	"os"
)

// MakeDir creates a directory if it does't exist yet and sets
// directory's mode and permission bits - e.g. 0775.
func MakeDir(path string, permissions os.FileMode) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0775); err != nil {
			return err
		}
	}

	return nil
}
