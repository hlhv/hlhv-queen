package srvhttps

import (
        "strconv"
        "net/http"
        "github.com/hlhv/scribe"
        "github.com/hlhv/hlhv/conf"
)

var (
        mux *HolaMux
        port string
)

func Arm () (err error) {
        port = strconv.Itoa(conf.GetPortHttps())
        scribe.PrintProgress("arming https server on port", port)
        mux = NewHolaMux()
        return nil
}

func Fire () {
        keyPath  := conf.GetKeyPath()
        certPath := conf.GetCertPath()
        exitMsg  := http.ListenAndServeTLS(":" + port, certPath, keyPath, mux)
        scribe.PrintFatal(exitMsg.Error())
}

func MountFunc (
        pattern string,
        handler func(http.ResponseWriter, *http.Request),
) (
        err error,
) {
        return mux.MountFunc(pattern, handler)
}

func Unmount (pattern string) (err error) {
        return mux.Unmount(pattern)
}
