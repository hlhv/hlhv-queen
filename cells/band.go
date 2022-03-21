package cells

import (
        "net"
        // "sync"
        // "net/http"
        // "encoding/json"
        "github.com/hlhv/fsock"
        // "github.com/hlhv/hlhv/scribe"
)

type Band struct {
        conn   net.Conn
        Reader *fsock.Reader
        Writer *fsock.Writer
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
                Reader: reader,
                Writer: writer,
                lock:   false,
        }
}

func (band *Band) Close () {
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
