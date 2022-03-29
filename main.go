package main

import (
        "github.com/hlhv/scribe"
        "github.com/hlhv/hlhv/conf"
        "github.com/hlhv/hlhv/srvhttps"
        "github.com/hlhv/hlhv/wrangler"
)

func main () {
        ParseArgs()

        go start()
        loop()
}

func start () {
        var err error

        if options.logLevel <= scribe.LogLevelNormal { printBanner() }
        scribe.PrintProgress(scribe.LogLevelNormal, "starting hlhv queen cell")

        err = conf.Load(options.confPath)
        if err != nil {
                scribe.PrintWarning (
                        scribe.LogLevelError,
                        "could not load conf: " + err.Error())
                scribe.PrintWarning (
                        scribe.LogLevelError,
                        "using default configuration")
        }

        err = wrangler.Arm()
        if err != nil {
                scribe.PrintFatal (
                        scribe.LogLevelError,
                        "could not arm wrangler: " + err.Error())
                return
        }
        err = srvhttps.Arm()
        if err != nil {
                scribe.PrintFatal (
                        scribe.LogLevelError,
                        "could not arm srvhttps: " + err.Error())
                return
        }

        scribe.PrintProgress(scribe.LogLevelNormal, "firing")
        go wrangler.Fire()
        go srvhttps.Fire()

        scribe.PrintDone (
                scribe.LogLevelNormal,
                "startup sequence complete, resuming normal operation")
}

func loop () {
        for {
                scribe.ListenOnce(options.logLevel)
        }
}
