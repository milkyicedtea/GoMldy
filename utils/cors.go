package utils

import (
	"log"
	"os"
	"regexp"
)

func RegexCORS(origin string) bool {
	var debug = os.Getenv("MODE") == "dev" || os.Getenv("MODE") == "debug"
	var originRegex *regexp.Regexp

	//log.Println(debug)

	if debug {
		log.Println("Using debug regex")
		originRegex = regexp.MustCompile(`(https?://)?(192)\.(168)\.(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]|[0-9]){2}(?::\d+)?|localhost(?::\d+)?|127.0.0.1(?::\d+)?`)
	} else {
		log.Println("Using prod regex")
		originRegex = regexp.MustCompile(`^(?:https?://(?:.*\.)?051205\.xyz(?::\d+)?|https?://[\w.-]+:\d+)$`)
	}

	matches := originRegex.MatchString(origin)
	log.Printf("Origin: %s, Matches: %t", origin, matches)
	return matches
}
