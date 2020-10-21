package filesystem

import "os"

// MakeDir creates a directory if it does't exist yet and sets
// directory's mode and permission bits - e.g. 0755.
func MakeDir(path string, permissions os.FileMode) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.Mkdir(path, permissions); err != nil {
			return err
		}
	}

	return nil
}
