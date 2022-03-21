package main

import (
        "github.com/hlhv/hlhv/conf"
        "github.com/hlhv/hlhv/scribe"
        "github.com/hlhv/hlhv/srvhttps"
        "github.com/hlhv/hlhv/wrangler"
)

func main () {
        go start()
        loop()
}

func start () {
        var err error

        printBanner()
        scribe.PrintProgress("starting hlhv queen cell")

        err = conf.Load()
        if err != nil {
                scribe.PrintWarning("could not load conf: " + err.Error())
                scribe.PrintWarning("using default configuration")
        }

        err = wrangler.Arm()
        if err != nil {
                scribe.PrintFatal("could not arm wrangler: " + err.Error())
                return
        }
        err = srvhttps.Arm()
        if err != nil {
                scribe.PrintFatal("could not arm srvhttps: " + err.Error())
                return
        }

        scribe.PrintProgress("firing")
        go wrangler.Fire()
        go srvhttps.Fire()

        scribe.PrintDone("startup sequence complete, resuming normal operation")
}

func loop () {
        for {
                scribe.ListenOnce()
        }
}
