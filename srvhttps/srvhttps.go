package srvhttps

import (
        "time"
        "strconv"
        "net/http"
        "crypto/tls"
        "github.com/hlhv/scribe"
        "github.com/hlhv/hlhv/conf"
)

var (
        mux    *HolaMux
        server *http.Server
        port   string
)

func Arm () (err error) {
        port = strconv.Itoa(conf.GetPortHttps())
        scribe.PrintProgress("arming https server on port", port)
        mux = NewHolaMux()

        // following:
        // https://blog.cloudflare.com/exposing-go-on-the-internet/
        serverConf := &tls.Config {
        	PreferServerCipherSuites: true,
        	CurvePreferences: []tls.CurveID {
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
        
        server = &http.Server {
                Addr:              ":" + port,
                ReadHeaderTimeout: 5 * time.Second,
                ReadTimeout:       10 * time.Second,
                WriteTimeout:      15 * time.Second,
                IdleTimeout:       120 * time.Second,
                TLSConfig:         serverConf,
                Handler:           mux,
        }
        return nil
}

func Fire () {
        keyPath  := conf.GetKeyPath()
        certPath := conf.GetCertPath()
        
        exitMsg  := server.ListenAndServeTLS(certPath, keyPath)
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
