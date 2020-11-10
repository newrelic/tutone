package output

import (
	"fmt"
)

// PrintSuccessMessage prints a message to the console informing
// the user that code generation was a success and outputs the
// package and file path for reference.
//
// Emoji unicode reference: http://www.unicode.org/emoji/charts/emoji-list.html
func PrintSuccessMessage(packagePath string, filePath string) {
	// Emoji = \u2705
	fmt.Print("\n\u2705 Code generation complete: \n\n")
	fmt.Printf("   Package:   %v \n", packagePath)
	fmt.Printf("   File:      %v \n", filePath)
	fmt.Println("")
}
