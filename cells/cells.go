package cells

import (
        "io"
        "fmt"
        "net"
        "sync"
        "time"
        "errors"
        "strings"
        "strconv"
        "net/http"
        "encoding/json"
        "container/list"
        "github.com/hlhv/fsock"
        "github.com/hlhv/scribe"
        "github.com/google/uuid"
        "github.com/hlhv/protocol"
        "github.com/hlhv/hlhv-queen/conf"
        "github.com/hlhv/hlhv-queen/srvhttps"
)

/* Cell represents a connection to a cell server. It should only be created in
 * response to an incoming tls connection, using the Handle function.
 */
type Cell struct {
        leash   net.Conn
        Reader  *fsock.Reader
        Writer  *fsock.Writer
        
        bands      *list.List
        bandsMutex sync.Mutex
        waitList   chan chan *Band
        
        mount string

        sigQueue chan Sig
        
        key     string
        uuid    string
        onClean func (*Cell)
}

func NewCell (
        leash      net.Conn,
        reader     *fsock.Reader,
        writer     *fsock.Writer,
        uuidString string,
        onClean    func (*Cell),
) (
        cell *Cell,
) {
        key := uuid.New()
        keyString := key.String()

        return &Cell {
                leash:    leash,
                Reader:   reader,
                Writer:   writer,
                bands:    list.New(),
                waitList: make(chan chan *Band, 64),
                sigQueue: make(chan Sig),
                key:      keyString,
                uuid:     uuidString,
                onClean:  onClean,
        }
}

/* listen listens for incoming data on the socket, and uses it to fulfill
* requests.
 */
func (cell *Cell) Listen () {
        for {
                kind, data, err := protocol.ReadParseFrame(cell.Reader)
                if err == io.EOF { break }
                if err != nil {
                        scribe.PrintError (
                                scribe.LogLevelError,
                                "error parsing frame, kicking cell:", err)
                        cell.leash.Close()
                        break
                }
                err = cell.handleOneFrame(kind, data)
                if err == io.EOF { break }
                if err != nil {
                        scribe.PrintError (
                                scribe.LogLevelError,
                                "error handling frame, kicking cell:", err)
                        cell.leash.Close()
                        break
                }
        }

        // the leash has closed, so clean up the cell
        scribe.PrintDisconnect(scribe.LogLevelNormal, "cell disconnected")
        cell.cleanUp()
}

func (cell *Cell) handleOneFrame (
        kind protocol.FrameKind,
        data []byte,
) (
        err error,
) {
        switch kind {
        case protocol.FrameKindMount:
                frame := protocol.FrameMount {}
                err = json.Unmarshal(data, &frame)
                if err != nil { return err }

                // mount
                pattern := frame.Host + frame.Path
                err = cell.MountFunc(pattern, cell.HandleHTTP)
                if err != nil { return err }
                break
        
        case protocol.FrameKindUnmount:
                // unmount
                cell.Unmount()
                if err != nil { return err }
                break

        default:
                return errors.New (fmt.Sprint (
                        "cell sent strange kind code on leash:", kind))
        }
        return nil
}

/* Key returns the cell's key
 */
func (cell *Cell) Key () string {
        return cell.key
}

/* Uuid returns the cell's uuid
 */
func (cell *Cell) Uuid () string {
        return cell.uuid
}

/* MountFunc is, for now, a wrapper around HolaMux.MountFunc().
 */
func (cell *Cell) MountFunc (
        pattern string,
        callback func(http.ResponseWriter, *http.Request),
) (
        err error,
) {
        err = srvhttps.MountFunc(pattern, callback)
        if err != nil { return err }

        // add to mounts
        cell.mount = pattern

        return nil
}

/* Unmount is, for now, a wrapper around HolaMux.Unmount().
 */
func (cell *Cell) Unmount () (err error) {
        if cell.mount == "" { errors.New("cell is not mounted") }

        err = srvhttps.Unmount(cell.mount)
        if err != nil { return err }
        cell.mount = ""
                
        return nil
}

/* HandleHTTP handles an http request directed at this cell. It selects a free
 * band, and then uses it to inform the cell of a new request and it pipes the
 * response back to the client.
 */
func (cell *Cell) HandleHTTP (
        res http.ResponseWriter,
        req *http.Request,
) {
        scribe.PrintInfo(scribe.LogLevelDebug, "handling http request")

        // build and normalize headers
        headers := make(map[string] []string)
        for key, value := range(req.Header) {
                // make all keys lowercase. if there are two instances of a key
                // with different cases, we need to combine them.
                lowerKey := strings.ToLower(key)
                headers[lowerKey] = append(headers[lowerKey], value...)
        }
        
        
        nPort, _ := strconv.Atoi(req.URL.Port())
        frameHead := &protocol.FrameHTTPReqHead {
                RemoteAddr: req.RemoteAddr,
                Method:     req.Method,
                Scheme:     req.URL.Scheme,
                Host:       req.URL.Hostname(),
                Port:       nPort,
                Path:       req.URL.Path,
                Fragment:   req.URL.Fragment,
                Query:      (map[string] []string)(req.URL.Query()),
                Proto:      req.Proto,
                ProtoMajor: req.ProtoMajor,
                ProtoMinor: req.ProtoMinor,
                Headers:    headers,
        }

        var band *Band
        var err  error

        defer func () {
                if band != nil { band.Unlock() }
        } ()

        // get a band, and use it to send the request to the cell. if it didn't
        // work, mark the band as closed and get a new one.
        scribe.PrintProgress(scribe.LogLevelDebug, "sending header to cell")
        for {
                band, err = cell.Provide()
                if err != nil {
                        err = errors.New(fmt.Sprint("server overload:", err))
                        scribe.PrintError(scribe.LogLevelError, err)
                        srvhttps.WriteServUnavail(res, req, err)
                        return
                }
         
                _, err = band.WriteMarshalFrame(frameHead)
                if err == nil { break }
                band.Close()
                scribe.PrintInfo (
                        scribe.LogLevelDebug,
                        "detected closed band, asking for new one")
                
        }

        // write body to cell
        scribe.PrintProgress(scribe.LogLevelDebug, "sending body to cell")
        bodyBuffer := make([]byte, 1024)
        for {
                scribe.PrintProgress(scribe.LogLevelDebug, "reading body chunk")
                bytesRead, err := req.Body.Read(bodyBuffer)
                if err != nil { break }
                
                scribe.PrintProgress (
                        scribe.LogLevelDebug,
                        "writing body chunk of size", bytesRead)
                
                _, err = band.writer.WriteFrame (
                        append (
                                []byte { byte(protocol.FrameKindHTTPReqBody) },
                                bodyBuffer[:bytesRead]...
                        ),
                )
                
                if err != nil {
                        band.Close()
                        err = errors.New (fmt.Sprint (
                                "band closed abruptly:", err))
                        scribe.PrintError(scribe.LogLevelError, err)
                        srvhttps.WriteBadGateway(res, req, err)
                        return
                }
        }

        // write end to cell
        scribe.PrintProgress(scribe.LogLevelDebug, "sending end to cell")
        _, err = band.WriteMarshalFrame(&protocol.FrameHTTPReqEnd {})
        if err != nil {
                band.Close()
                err = errors.New(fmt.Sprint("band closed abruptly: ", err))
                scribe.PrintError(scribe.LogLevelError, err)
                srvhttps.WriteBadGateway(res, req, err)
                return
        }

        // read head from cell
        scribe.PrintProgress(scribe.LogLevelDebug, "reading head from cell")
        kind, data, err := band.ReadParseFrame()
        if err != nil {
                band.Close()
                scribe.PrintError(scribe.LogLevelError, err)
                srvhttps.WriteBadGateway(res, req, err)
                return
        }

        if kind != protocol.FrameKindHTTPResHead {
                band.Close()
                err = errors.New (fmt.Sprint (        
                        "band sent unknown code ", kind, ", expecting response",
                        "head"))
                scribe.PrintError(scribe.LogLevelError, err)
                srvhttps.WriteBadGateway(res, req, err)
                return
        }

        // parse head
        resHead := &protocol.FrameHTTPResHead {}
        err = json.Unmarshal(data, resHead)

        if resHead.StatusCode < 200 {
                err = errors.New (fmt.Sprint (        
                        "band sent bad status code ", resHead.StatusCode))
                scribe.PrintError(scribe.LogLevelError, err)
                srvhttps.WriteBadGateway(res, req, err)
                return
        }

        // write headers
        scribe.PrintProgress(scribe.LogLevelDebug, "sending head")
        for key, values := range(resHead.Headers) {
                // each key may have multiple values
                for _, value := range(values) {
                        res.Header().Add(key, value)
                }
        }
        // write status code
        res.WriteHeader(resHead.StatusCode)

        // pipe body from cell to client
        scribe.PrintProgress(scribe.LogLevelDebug, "piping body from cell")
        for {
                kind, data, err := band.ReadParseFrame()
                if err != nil {
                        band.Close()
                        err = errors.New (fmt.Sprint (
                                "band closed abruptly: ", err))
                        scribe.PrintError(scribe.LogLevelError, err)
                        srvhttps.WriteBadGateway(res, req, err)
                        return
                }

                if kind == protocol.FrameKindHTTPResEnd {
                        scribe.PrintDone (
                                scribe.LogLevelDebug,
                                "http request done")
                        return
                }

                if kind != protocol.FrameKindHTTPResBody {
                band.Close()
                        err = errors.New (fmt.Sprint (        
                                "band sent unknown code ", kind, ", expecting",
                                "response body"))
                        scribe.PrintError(scribe.LogLevelError, err)
                        srvhttps.WriteBadGateway(res, req, err)
                        return
                }

                _, err = res.Write(data)
                if err != nil {
                        err = errors.New (fmt.Sprint (        
                                "http request mysteriously died: ", err))
                        scribe.PrintError(scribe.LogLevelError, err)
                        return
                }
        }

        return
}

/* Bind adds a band to the cell, and fulfils a pending request for more.
 */
func (cell *Cell) Bind (band *Band, key string) (err error) {
        if key != cell.key {
                return errors.New (fmt.Sprint (
                        "band sent strange key: ", key,
                ))
        }

        cell.bandsMutex.Lock()
        cell.bands.PushBack(band)
        cell.bandsMutex.Unlock()

        select {
        case request := <- cell.waitList:
                scribe.PrintInfo (
                        scribe.LogLevelDebug,
                        "found band request, fulfilling")
                request <- band
                break
        default:
                scribe.PrintInfo (
                        scribe.LogLevelDebug,
                        "no band requests to fulfill")
                break
        }

        return nil
}

/* Provide returns an unlocked band that is not currently being used. If it
 * can't find one, it puts in a request for one and waits until it is available.
 * The band must be manually re-locked after use! (except on error)
 */
func (cell *Cell) Provide () (band *Band, err error) {
        // try to find a free band, and while we're at it, remove ones that have
        // been marked as closed.
        cell.bandsMutex.Lock()
        item := cell.bands.Front()
        for item != nil {
                band := item.Value.(*Band)
                if band.open && band.TryLock() {
                        cell.bandsMutex.Unlock()
                        return band, nil
                }
                item = item.Next()
        }
        cell.bandsMutex.Unlock()

        // else, put in a request for a new one and wait
        // request the next band be sent to us
        scribe.PrintInfo(scribe.LogLevelDebug, "new band needed")
        request := make(chan *Band)
        cell.waitList <- request
        scribe.PrintInfo(scribe.LogLevelDebug, "request made")
        // send a request to the cell for a new band
        cell.SendSig(SigNeedBand)
        // wait for request to be fulfilled
        scribe.PrintProgress(scribe.LogLevelDebug, "waiting for fulfill")
        band = <- request
        scribe.PrintDone(scribe.LogLevelDebug, "band request fulfilled")

        if band == nil {
                return nil, errors.New (
                        "internal communication bandwidth exceeded")
        }

        return band, nil
}

/* Prune removes bands that haven't been used in a while, or have been marked as
 * closed.
 */
func (cell *Cell) Prune () (pruned int) {
        cell.bandsMutex.Lock()
        defer cell.bandsMutex.Unlock()
        
        maxBandAge := time.Duration(conf.GetMaxBandAge()) * time.Second

        now := time.Now()
        threshold := now.Add(-1 * maxBandAge)
        
        item := cell.bands.Front()
        for item != nil {
                band := item.Value.(*Band)
                
                if band.lastUsed.Before(threshold) {
                        band.Close()
                }
                
                if !band.open {
                        cell.bands.Remove(item)
                        pruned ++
                }
                
                item = item.Next()
        }

        return
}

/* cleanUp should be called when the leash closes, and only when the leash
 * closes. It calls the externally specified cleanup function (which should
 * remove the cell from a server-wide cell list), unmounts the cell, and shuts
 * down all bands.
 */
func (cell *Cell) cleanUp () {
        scribe.PrintProgress(scribe.LogLevelDebug, "cleaning up cell")
        cell.onClean(cell)

        // stop listening for signals
        cell.SendSig(SigCleaning)

        // unmount
        cell.Unmount()
        
        // close all bands immediately
        cell.bandsMutex.Lock()
        item := cell.bands.Front()
        for item != nil {
                item.Value.(*Band).Close()
                item = item.Next()
        }
        cell.bandsMutex.Unlock()
        scribe.PrintDone(scribe.LogLevelDebug, "cleaned up cell")
}
