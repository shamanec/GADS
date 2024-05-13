package util

import (
	"fmt"
	"os"
)

func ExitWithErrorMessage(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}
