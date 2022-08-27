package main

import (
	"os"
	"syscall"
	"os/signal"
	"github.com/hlhv/hlhv-queen/conf"
	"github.com/hlhv/hlhv-queen/srvhttps"
	"github.com/hlhv/hlhv-queen/wrangler"
	"github.com/hlhv/scribe"
)

func main() {
	ParseArgs()
	scribe.SetLogLevel(options.logLevel)

	arm()
	fire()

	// create sigint handler
	sigintNotify := make(chan os.Signal, 1)
	signal.Notify(sigintNotify, os.Interrupt, syscall.SIGTERM)
	<- sigintNotify
	scribe.PrintProgress(scribe.LogLevelNormal, "shutting down")

	// TODO: if in the future the queen cell needs to do any shutdown
	// processes, do so here.
		
	scribe.PrintDone(scribe.LogLevelNormal, "exiting")
	scribe.Stop()
	os.Exit(0)
}

func arm() {
	var err error

	scribe.PrintProgress(scribe.LogLevelNormal, "starting hlhv queen cell")

	err = conf.Load(options.confPath)
	if err != nil {
		scribe.PrintWarning(
			scribe.LogLevelError,
			"could not load conf: "+err.Error())
		scribe.PrintWarning(
			scribe.LogLevelError,
			"using default configuration")
	}

	err = wrangler.Arm()
	if err != nil {
		scribe.PrintFatal(
			scribe.LogLevelError,
			"could not arm wrangler: "+err.Error())
		return
	}
	err = srvhttps.Arm()
	if err != nil {
		scribe.PrintFatal(
			scribe.LogLevelError,
			"could not arm srvhttps: "+err.Error())
		return
	}
}

func fire() {
	scribe.PrintProgress(scribe.LogLevelNormal, "firing")
	go wrangler.Fire()
	go srvhttps.Fire()

	scribe.PrintDone(
		scribe.LogLevelNormal,
		"startup sequence complete, resuming normal operation")
}
