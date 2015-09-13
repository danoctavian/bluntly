package node

import (
	"fmt"
	"log"
)

const debugLevel = LOG_DEBUG

const (
	LOG_DEBUG  = 4
	LOG_INFO   = 3
	LOG_WARN   = 2
	LOG_ERROR  = 1
)

var levels = []string{
	"error",
	"warn",
	"info",
	"debug",
}

func Log(level byte, format string, args ...interface{}) {
	if level <= debugLevel {
		text := fmt.Sprintf(format, args...)
		log.Print(levels[level-1], " ", text)
	}
}
