package srvhttps

import (
        "net/http"
        "github.com/hlhv/scribe"
)

func WriteSysmsg (
        res http.ResponseWriter,
        req *http.Request,
        code int,
        title string,
        content string,
) {
        res.WriteHeader(code)
        _, err := res.Write ([]byte (
                "<!DOCTYPE html><html><head><title>" + title + "</title>" +
                "<meta name=\"viewport\" content=\"width=device-width, " +
                "initial-scale=1.0\"><style>" +
                "body{font-family:monospace;max-width:512px;margin:4em auto;" +
                "background-color:#2b303c;color:#eceff4}" +
                "hr{border:1px solid #4c566a;width:128px;margin:0}" +
                "*::selection{background-color:#4c566a}" +
                "</style></head><body>" +
                "<h1>" + title + "</h1><hr>" +
                "<p>hlhv system message:</p>" +
                "<p>" + content + "</p>" +
                "</body></html>",
        ))
        if err != nil { scribe.PrintError("cannot write sysmsg:", err) }
}

func NotFoundHandler () http.Handler {
        return http.HandlerFunc(WriteNotFound)
}

func WriteNotFound (res http.ResponseWriter, req *http.Request) {
        WriteSysmsg (
                res, req, 404,
                "404 - not found",
                "ERR there is no cell mounted on the path " +
                req.URL.Path)
}

func WriteBadGateway (res http.ResponseWriter, req *http.Request, err error) {
        WriteSysmsg (
                res, req, 502,
                "502 - bad gateway",
                "ERR cell couldn't handle http req: " +
                err.Error())
}

func WriteServUnavail (res http.ResponseWriter, req *http.Request, err error) {
        WriteSysmsg (
                res, req, 503,
                "503 - service unavailable",
                "ERR this page is unavailable right now: " +
                err.Error())
}


func WritePlaceholder (res http.ResponseWriter, req *http.Request) {
        WriteSysmsg (
                res, req, 200,
                "under construction",
                "(i) this page has not been built yet. check back later!")
}
