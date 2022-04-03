package srvhttps

import (
        "net"
        "sync"
        "sort"
        "path"
        "errors"
        "strings"
        "net/url"
        "net/http"
        "github.com/hlhv/scribe"
        "github.com/hlhv/hlhv-queen/conf"
)

/* This code was originally taken from the http package source, and modified to
 * suit the needs of hlhv (as well as heavily refactored and cleaned). If
 * something breaks, it may be helpful to refer to:
 * https://cs.opensource.google/go/go/+/refs/tags/go1.18:src/net/http/server.go
 */

/* HolaMux is a custom http request multiplexer implementation based the default
 * golang ServeMux. It is designed to better integrate with hlhv.
 */
type HolaMux struct {
        mutex         sync.RWMutex
        exactEntries  map[string]muxEntry
        sortedEntries []muxEntry // slice of entries sorted from longest to shortest.
}

type muxEntry struct {
        handler http.Handler
        pattern string
}

/* NewHolaMux allocates and returns a new HolaMux.
 */
func NewHolaMux () *HolaMux { return new(HolaMux) }

/* cleanPath returns the canonical path for p, eliminating . and .. elements.
 */
func cleanPath (p string) string {
        if p == "" {
                return "/"
        }
        
        if p[0] != '/' {
                p = "/" + p
        }
        
        np := path.Clean(p)
        // path.Clean removes trailing slash except for root;
        // put the trailing slash back if necessary.
        if p[len(p) - 1] == '/' && np != "/" {
                // Fast path for common case of p being the string we want:
                if len(p) == len(np) + 1 && strings.HasPrefix(p, np) {
                        np = p
                } else {
                        np += "/"
                }
        }
        
        return np
}

/* stripHostPort returns h without any trailing ":<port>".
 */
func stripHostPort (h string) string {
        // If no port on host, return unchanged
        if !strings.Contains(h, ":") {
                return h
        }
        
        host, _, err := net.SplitHostPort(h)
        if err != nil {
                return h // on error, return unchanged
        }
        
        return host
}

/* Find a handler on a handler map given a path string.
 * Most-specific (longest) pattern wins.
 */
func (mux *HolaMux) match (path string) (h http.Handler, pattern string) {
        // Check for exact match first.
        entry, matchExists := mux.exactEntries[path]
        if matchExists {
                return entry.handler, entry.pattern
        }

        // Check for longest valid match.  mux.es contains all patterns
        // that end in / sorted from longest to shortest.
        for _, entry := range mux.sortedEntries {
                if strings.HasPrefix(path, entry.pattern) {
                        return entry.handler, entry.pattern
                }
        }
        
        return nil, ""
}

/* redirectToPathSlash determines if the given path needs appending "/" to it.
 * This occurs when a handler for path + "/" was already registered, but
 * not for path itself. If the path needs appending to, it creates a new
 * URL, setting the path to u.Path + "/" and returning true to indicate so.
 */
func (mux *HolaMux) redirectToPathSlash (
        host,
        path string,
        u *url.URL,
) (
        *url.URL,
        bool,
) {
        mux.mutex.RLock()
        shouldRedirect := mux.shouldRedirectRLocked(host, path)
        mux.mutex.RUnlock()
        
        if !shouldRedirect {
                return u, false
        }
        
        path = path + "/"
        u = &url.URL { Path: path, RawQuery: u.RawQuery }
        return u, true
}

/* shouldRedirectRLocked reports whether the given path and host should be
 * redirected to path+"/". This should happen if a handler is registered for
 * path+"/" but not path -- see comments at ServeMux.
 */
func (mux *HolaMux) shouldRedirectRLocked (host, path string) bool {
        pathTry := host + path

        // check if path is handled
        _, exists := mux.exactEntries[pathTry]
        if exists {
                return false
        }

        // if path is empty don't even bother
        pathLen := len(path)
        if pathLen == 0 {
                return false
        }

        // check if path/ is handled
        _, exists = mux.exactEntries[pathTry + "/"]
        if exists {
                return path[pathLen - 1] != '/'
        }

        return false
}

/* Handler returns the handler to use for the given request, consulting
 * r.Method, r.Host, and r.URL.Path. It always returns a non-nil handler. If the
 * path is not in its canonical form, the handler will be an
 * internally-generated handler that redirects to the canonical path. If the
 * host contains a port, it is ignored  when matching handlers.
 *
 * The path and host are used unchanged for CONNECT requests.
 *
 * Handler also returns the registered pattern that matches the request or, in
 * the case of internally-generated redirects, the pattern that will match after
 * following the redirect.
 *
 * If there is no registered handler that applies to the request, Handler
 * returns a ``page not found'' handler and an empty pattern.
 */
func (mux *HolaMux) Handler (r *http.Request) (h http.Handler, pattern string) {
        // TODO: possibly remove all CONNECT support, because hlhv is not
        // designed to be a raw socket proxy.
        // CONNECT requests are not canonicalized.
        if r.Method == "CONNECT" {
                // If r.URL.Path is /tree and its handler is not registered,
                // the /tree -> /tree/ redirect applies to CONNECT requests
                // but the path canonicalization does not.
                u, shouldRedirect := mux.redirectToPathSlash (
                        r.URL.Host,
                        r.URL.Path,
                        r.URL)
                if shouldRedirect {
                        return http.RedirectHandler (
                                        u.String(),
                                        http.StatusMovedPermanently),
                                u.Path
                }

                return mux.handlerInternal(r.Host, r.URL.Path)
        }

        // All other requests have any port stripped and path cleaned
        // before passing to mux.handler.
        host := stripHostPort(r.Host)
        path := cleanPath(r.URL.Path)

        // resolve hostname aliases if there are any
        host = conf.ResolveAliases(host)
        scribe.PrintResolve (
                scribe.LogLevelDebug,
                "resolved to \"" + host + path + "\"")

        // If the given path is /tree and its handler is not registered,
        // redirect for /tree/.
        u, shouldRedirect := mux.redirectToPathSlash(host, path, r.URL)
        if shouldRedirect {
                return http.RedirectHandler (
                                u.String(),
                                http.StatusMovedPermanently),
                        u.Path
        }

        if path != r.URL.Path {
                _, pattern = mux.handlerInternal(host, path)
                u := &url.URL{Path: path, RawQuery: r.URL.RawQuery}
                return http.RedirectHandler (
                                u.String(),
                                http.StatusMovedPermanently),
                        pattern
        }

        return mux.handlerInternal(host, r.URL.Path)
}

/* handlerInternal is the main implementation of Handler.
 * The path is known to be in canonical form, except for CONNECT methods.
 */
func (mux *HolaMux) handlerInternal (
        host,
        path string,
) (
        h http.Handler,
        pattern string,
) {
        mux.mutex.RLock()
        defer mux.mutex.RUnlock()
        
        // all patterns are host specific
        h, pattern = mux.match(host + path)
        
        if h == nil {
                scribe.PrintError(scribe.LogLevelError,"404", pattern)
                h, pattern = NotFoundHandler(), ""
        }
        
        return
}

/* ServeHTTP dispatches the request to the handler whose
 * pattern most closely matches the request URL.
 */
func (mux *HolaMux) ServeHTTP (w http.ResponseWriter, r *http.Request) {
        scribe.PrintRequest (
                scribe.LogLevelNormal,
                "request for \"" + r.Host + r.URL.Path + "\" by", r.RemoteAddr)
        
        if r.RequestURI == "*" {
                if r.ProtoAtLeast(1, 1) {
                        w.Header().Set("Connection", "close")
                }
                w.WriteHeader(http.StatusBadRequest)
                return
        }
        
        h, _ := mux.Handler(r)
        h.ServeHTTP(w, r)
}

/* Mount registers the handler for the given pattern, resolving all aliases. If
 * the pattern is already registered, or the pattern is invalid, Mount returns
 * an error. If the pattern ends in a '/', it will match all unregistered
 * subpatterns.
 */
func (mux *HolaMux) Mount (pattern string, handler http.Handler) error {
        mux.mutex.Lock()
        defer mux.mutex.Unlock()

        if pattern == "" {
                return errors.New (
                        "mux: invalid pattern " + pattern +
                        ", cannot be empty.")
        }

        if pattern[0] == '/' {
                return errors.New (
                        "mux: invalid pattern " + pattern +
                        ", must be host specific.")
        }
        
        if handler == nil {
                return errors.New("mux: nil handler")
        }
        
        if _, exist := mux.exactEntries[pattern]; exist {
                return errors.New("mux: existing mount on " + pattern)
        }

        if mux.exactEntries == nil {
                mux.exactEntries = make(map[string]muxEntry)
        }
        
        entry := muxEntry { handler: handler, pattern: pattern }
        mux.exactEntries[pattern] = entry
        if pattern[len(pattern) - 1] == '/' {
                mux.sortedEntries = appendSorted(mux.sortedEntries, entry)
        }

        scribe.PrintMount(scribe.LogLevelNormal, "mount on", pattern)
        return nil
}

func (mux *HolaMux) MountFunc (
        pattern string,
        handler func(http.ResponseWriter, *http.Request),
) (
        err error,
) {
	if handler == nil {
		return errors.New("mux: nil handler")
	}
	mux.Mount(pattern, http.HandlerFunc(handler))
	return nil
}

/* Unmount removes a registry, and returns an error on fail.
 */
func (mux *HolaMux) Unmount (pattern string) error {
        mux.mutex.Lock()
        defer mux.mutex.Unlock()

        // delete from exact match list
        if _, registered := mux.exactEntries[pattern]; !registered {
                return errors.New (
                        "mux: pattern " + pattern + " is not mounted")
        }
        delete(mux.exactEntries, pattern)

        // delete from sorted list, if its in there.
        newLen := 0
        for index, entry := range(mux.sortedEntries) {
                if entry.pattern != pattern {
                        mux.sortedEntries[newLen] = mux.sortedEntries[index]
                        newLen ++
                }
        }
        mux.sortedEntries = mux.sortedEntries[:newLen]
        
        scribe.PrintUnmount(scribe.LogLevelNormal, "unmount from", pattern)
        return nil
}

func appendSorted (entries []muxEntry, entry muxEntry) []muxEntry {
        entriesLen := len(entries)
        index := sort.Search(entriesLen, func(index int) bool {
                return len(entries[index].pattern) < len(entry.pattern)
        })
        
        if index == entriesLen {
                return append(entries, entry)
        }
        
        // we now know that i points at where we want to insert
        // try to grow the slice in place, any entry works
        entries = append(entries, muxEntry{})
        copy(entries[index + 1:], entries[index:]) // Move shorter entries down
        entries[index] = entry
        return entries
}
