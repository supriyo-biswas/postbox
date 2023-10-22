package smtp

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

var commandRe = regexp.MustCompile(`\s+`)
var emailArgsRe = regexp.MustCompile(`(\w+)\s*:\s*<\s*(.*)\s*>`)
var errCredentialDecode = fmt.Errorf("failed to decode credentials")

var errInvalidSyntax = fmt.Errorf("invalid syntax")

func parseCommand(message string) (string, string) {
	parts := commandRe.Split(strings.TrimRight(message, " \t\r\n"), 2)
	if len(parts) == 0 {
		return "", ""
	}

	parts[0] = strings.ToUpper(parts[0])
	if len(parts) == 1 {
		return parts[0], ""
	}

	return parts[0], parts[1]
}

func parseEmailArgs(args string) (string, string, error) {
	parts := emailArgsRe.FindStringSubmatch(args)
	if len(parts) == 0 || !strings.Contains(parts[2], "@") {
		return "", "", errInvalidSyntax
	}
	return strings.ToUpper(parts[1]), parts[2], nil
}

func parsePlainCreds(args string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(args)
	if err != nil {
		return "", "", errCredentialDecode
	}

	parts := strings.SplitN(string(decoded), "\x00", 3)
	if len(parts) != 3 {
		return "", "", errCredentialDecode
	}

	return parts[1], parts[2], nil
}
