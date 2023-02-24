package output

import (
	"fmt"
)

// PrintSuccessMessage prints a message to the console informing
// the user that code generation was a success and outputs the
// package and file path for reference.
//
// Emoji unicode reference: http://www.unicode.org/emoji/charts/emoji-list.html
func PrintSuccessMessage(packagePath string, filePaths []string) {
	// Emoji = \u2705
	fmt.Print("\n\u2705 Code generation complete: \n\n")
	fmt.Printf("   Package:   %v \n", packagePath)

	for _, f := range filePaths {
		fmt.Printf("   File:      %v \n", f)
	}

	fmt.Println("")
}
