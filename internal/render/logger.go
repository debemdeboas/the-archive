package render

import "github.com/rs/zerolog"

var renderLogger zerolog.Logger

func SetLogger(l zerolog.Logger) {
	renderLogger = l
}
