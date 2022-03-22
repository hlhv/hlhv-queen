package cells

import (
        "net"
        // "sync"
        // "net/http"
        // "encoding/json"
        "github.com/hlhv/fsock"
        "github.com/hlhv/protocol"
        // "github.com/hlhv/hlhv/scribe"
)

type Band struct {
        conn   net.Conn
        reader *fsock.Reader
        writer *fsock.Writer
        open   bool
        lock   bool
}

func NewBand (
        conn net.Conn,
        reader *fsock.Reader,
        writer *fsock.Writer,
) (
        band *Band,
) {
        return &Band {
                conn:   conn,
                reader: reader,
                writer: writer,
                open:   true,
                lock:   false,
        }
}

/* ReadParseFrame reads a single frame and parses it, separating the kind and
 * the data.
 */
func (band *Band) ReadParseFrame () (
        kind protocol.FrameKind,
        data []byte,
        err error,
) {
        kind, data, err = protocol.ReadParseFrame(band.reader)
        if err != nil { band.Close() }
        return
}

/* WriteMarshalFrame marshals and writes a Frame.
 */
func (band *Band) WriteMarshalFrame (frame protocol.Frame) (nn int, err error) {
        nn, err = protocol.WriteMarshalFrame(band.writer, frame)
        if err != nil { band.Close() }
        return
}

func (band *Band) Close () {
        band.open = false
        band.conn.Close()
}

func (band *Band) TryLock () bool {
        /* this will not cause a race condition, because only one routine is
         * allowed to walk the band list at any given moment.
         */
        if band.lock { return false }
        band.lock = true
        return true
}

func (band *Band) Unlock () {
        band.lock = false
}
