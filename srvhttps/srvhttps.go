package srvhttps

import (
	"crypto/tls"
	"github.com/hlhv/hlhv-queen/conf"
	"github.com/hlhv/scribe"
	"net/http"
	"strconv"
	"time"
)

var mux *HolaMux
var server *http.Server
var port string
var stopNotify chan int
var listening bool

func Arm() (err error) {
	port = strconv.Itoa(conf.GetPortHttps())
	timeoutReadHeader := time.Duration(conf.GetTimeoutReadHeader())
	timeoutRead := time.Duration(conf.GetTimeoutRead())
	timeoutWrite := time.Duration(conf.GetTimeoutWrite())
	timeoutIdle := time.Duration(conf.GetTimeoutIdle())
	scribe.PrintProgress(
		scribe.LogLevelNormal,
		"arming https server on port", port)
	mux = NewHolaMux()

	// following:
	// https://blog.cloudflare.com/exposing-go-on-the-internet/
	serverConf := &tls.Config{
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	server = &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: timeoutReadHeader * time.Second,
		ReadTimeout:       timeoutRead * time.Second,
		WriteTimeout:      timeoutWrite * time.Second,
		IdleTimeout:       timeoutIdle * time.Second,
		TLSConfig:         serverConf,
		Handler:           mux,
	}
	return nil
}

func Fire() {
	scribe.PrintInfo(
		scribe.LogLevelDebug,
		"srvhttps listening")
	listening = true
	defer func() {
		listening = false
		scribe.PrintInfo(
			scribe.LogLevelDebug,
			"srvhttps no longer listening")
	}()

	keyPath := conf.GetKeyPath()
	certPath := conf.GetCertPath()
	exitMsg := server.ListenAndServeTLS(certPath, keyPath)

	if stopNotify == nil {
		scribe.PrintFatal(scribe.LogLevelError, exitMsg.Error())
	} else {
		stopNotify <- 0
	}
}

func Close() {
	if !listening {
		return
	}

	scribe.PrintProgress(scribe.LogLevelNormal, "stopping https server")
	stopNotify = make(chan int)
	server.Close()
	<-stopNotify
	scribe.PrintDone(scribe.LogLevelNormal, "stopped https server")
}

func MountFunc(
	pattern string,
	handler func(http.ResponseWriter, *http.Request),
) (
	err error,
) {
	return mux.MountFunc(pattern, handler)
}

func Unmount(pattern string) (err error) {
	return mux.Unmount(pattern)
}
