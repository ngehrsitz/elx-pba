package authentication

import (
	"fmt"
	"log"

	"github.com/u-root/u-root/pkg/ulog"
	"golang.org/x/term"
)

type PasswordAuthenticator struct{}

func (p PasswordAuthenticator) RetrievePassword() (string, error) {
	// avoid kernel log messages messing up prompt
	logErr := ulog.KernelLog.SetConsoleLogLevel(ulog.KLogWarning)
	if logErr != nil {
		log.Printf("Could not set log level KLogWarning: %v", logErr)
	}

	fmt.Println()
	fmt.Printf("Enter OPAL drive password (empty to skip): ")
	bytePassword, pwErr := term.ReadPassword(0)
	fmt.Println()
	if pwErr != nil {
		return "", fmt.Errorf("failed to read password from terminal: %v", pwErr)
	}
	return string(bytePassword), nil
}
