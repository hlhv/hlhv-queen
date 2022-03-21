package cells

import (
        "io"
        "fmt"
        "net"
        "sync"
        "errors"
        "net/http"
        "encoding/json"
        "container/list"
        "github.com/hlhv/fsock"
        "github.com/hlhv/protocol"
        "github.com/hlhv/hlhv/scribe"
        "github.com/hlhv/hlhv/srvhttps"
)

/* Cell represents a connection to a cell server. It should only be created in
 * response to an incoming tls connection, using the Handle function.
 */
type Cell struct {
        leash   net.Conn
        Reader  *fsock.Reader
        Writer  *fsock.Writer
        
        bands      []*Band
        bandsMutex sync.Mutex
        waitList   chan chan *Band
        
        mounts *list.List

        sigQueue chan Sig
        
        uuid    string
        onClean func (*Cell)
}

func NewCell (
        leash   net.Conn,
        reader  *fsock.Reader,
        writer  *fsock.Writer,
        onClean func (*Cell),
) (
        cell *Cell,
) {
        return &Cell {
                leash:    leash,
                Reader:   reader,
                Writer:   writer,
                waitList: make(chan chan *Band, 64),
                mounts:   list.New(),
                sigQueue: make(chan Sig),
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
                        scribe.PrintError("error parsing frame:", err)
                        continue
                }
                err = cell.handleOneFrame(kind, data)
                if err == io.EOF { break }
                if err != nil {
                        scribe.PrintError("error handling frame:", err)
                        continue
                }
        }

        // the leash has closed, so clean up the cell
        scribe.PrintDisconnect("cell disconnected")
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
                err = cell.MountFunc (pattern, func (
                        res http.ResponseWriter,
                        req *http.Request,
                ) {
                        cell.HandleHTTP(res, req)
                        
                })
                if err != nil { return err }
                break
        
        case protocol.FrameKindUnmount:
                frame := protocol.FrameUnmount {}
                err = json.Unmarshal(data, &frame)
                if err != nil { return err }

                // unmount
                pattern := frame.Host + frame.Path
                cell.Unmount(pattern)
                if err != nil { return err }
                break

        default:
                return errors.New (fmt.Sprint (
                        "cell sent strange kind code on leash:", kind))
        }
        return nil
}

/* MountFunc is, for now, a wrapper around HolaMux.MountFunc().
 */
func (cell *Cell) MountFunc (
        pattern string,
        callback func(res http.ResponseWriter, req *http.Request),
) (
        err error,
) {
        err = srvhttps.MountFunc(pattern, callback)
        if err != nil { return err }

        // add to mounts
        cell.mounts.PushBack(pattern)

        return nil
}

/* Unmount is, for now, a wrapper around HolaMux.Unmount().
 */
func (cell *Cell) Unmount (pattern string) (err error) {
        err = srvhttps.Unmount(pattern)
        if err != nil { return err }

        // remove from mounts
        curr := cell.mounts.Front()
        for curr != nil {
                if curr.Value.(string) == "" {
                        cell.mounts.Remove(curr)
                        break
                }
                curr = curr.Next()
        }
                
        return nil
}

func (cell *Cell) HandleHTTP (
        res http.ResponseWriter,
        req *http.Request,
) {
        band, err := cell.Provide()
        scribe.PrintInfo("handling http request")
        if err != nil {
                scribe.PrintError("server overload:", err)
                srvhttps.WriteServUnavail(res, req, err)
                return
        }

        srvhttps.WritePlaceholder(res, req)
        // TODO: perform http request

        band.Unlock()
        return
}

/* Uuid returns the uuid of a cell.
 */
func (cell *Cell) Uuid () string {
        return cell.uuid
}

/* Bind adds a band to the cell, and fulfils a pending request for more.
 */
func (cell *Cell) Bind (band *Band) {
        cell.bandsMutex.Lock()
        cell.bands = append(cell.bands, band)
        cell.bandsMutex.Unlock()

        select {
        case request := <- cell.waitList:
                scribe.PrintInfo("found band request, fulfilling")
                request <- band
                break
        default:
                scribe.PrintInfo("no band requests to fulfill")
                break
        }
        
}

/* Provide returns an unlocked band that is not currently being used. If it
 * can't find one, it puts in a request for one and waits until it is available.
 * The band must be manually re-locked after use! (except on error)
 */
func (cell *Cell) Provide () (band *Band, err error) {
        // try to find a free band
        cell.bandsMutex.Lock()
        for _, band := range(cell.bands) {
                if band.TryLock() {
                        cell.bandsMutex.Unlock()
                        return band, nil
                }
        }
        cell.bandsMutex.Unlock()

        // else, put in a request for a new one and wait
        // request the next band be sent to us
        scribe.PrintInfo("new band needed")
        request := make(chan *Band)
        cell.waitList <- request
        scribe.PrintInfo("request made")
        // send a request to the cell for a new band
        cell.SendSig(SigNeedBand)
        // wait for request to be fulfilled
        scribe.PrintProgress("waiting for fulfill")
        band = <- request
        scribe.PrintDone("band request fulfilled")

        if band == nil {
                return nil, errors.New (
                        "internal communication bandwidth exceeded")
        }

        return band, nil
}

/* cleanUp should be called when the leash closes, and only when the leash
 * closes. It calls the externally specified cleanup function (which should
 * remove the cell from a server-wide cell list), unmounts all mounts, and
 * shuts down all bands.
 */
func (cell *Cell) cleanUp () {
        scribe.PrintProgress("cleaning up cell")
        cell.onClean(cell)

        // stop listening for signals
        cell.SendSig(SigCleaning)

        // unmount all
        mount := cell.mounts.Front()
        for mount != nil {
                srvhttps.Unmount(mount.Value.(string))
                mount = mount.Next()
        }
        
        // close all bands immediately
        cell.bandsMutex.Lock()
        for _, band := range(cell.bands) {
                band.Close()
        }
        cell.bandsMutex.Unlock()
        scribe.PrintDone("cleaned up cell")
}
