package main

import (
	"fmt"
	"github.com/hlhv/scribe"
	"os"
	// TODO: write custom implementation
	"github.com/akamensky/argparse"
)

var options struct {
	logLevel     scribe.LogLevel
	confPath     string
	logDirectory string
}

func ParseArgs() {
	parser := argparse.NewParser("", "HLHV queen cell server")
	logLevel := parser.Selector("l", "log-level", []string{
		"debug",
		"normal",
		"error",
		"none",
	}, &argparse.Options{
		Required: false,
		Default:  "normal",
		Help: "The amount of logs to produce. Debug prints " +
			"everything, and none prints nothing",
	})

	logDirectory := parser.String("L", "log-directory", &argparse.Options{
		Required: false,
		Help: "The directory in which to store log files. If " +
			"unspecified, logs will be written to stdout",
	})


	confPath := parser.String("", "conf-path", &argparse.Options{
		Required: false,
		Default:  "/etc/hlhv/hlhv.conf",
		Help:     "Path to the config file",
	})

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	switch *logLevel {
	case "debug":
		options.logLevel = scribe.LogLevelDebug
		break
	default:
	case "normal":
		options.logLevel = scribe.LogLevelNormal
		break
	case "error":
		options.logLevel = scribe.LogLevelError
		break
	case "none":
		options.logLevel = scribe.LogLevelNone
		break
	}

	options.confPath = *confPath

	options.logDirectory = *logDirectory
	if options.logDirectory != "" {
		scribe.SetLogDirectory(options.logDirectory)
	}
}
