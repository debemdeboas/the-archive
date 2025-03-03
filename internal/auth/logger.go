package auth

import "github.com/rs/zerolog"

var authLogger zerolog.Logger

func SetLogger(l zerolog.Logger) {
	authLogger = l
}
