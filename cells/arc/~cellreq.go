// package cells
// type cellReqKind int
// 
// const (
        // cellKindReqDying cellReqKind = iota
        // cellKindReqHTTP
// )
// 
type CellReq interface {
        Kind() cellReqKind
}

type cellReqDying struct { }

func (req *cellReqDying) Kind () cellReqKind {
        return cellKindReqDying
}

/* run listens for requests, and writes them to the socket.
 */
func (cell *Cell) run () {
        for {
                req := <- cell.channel
                if !cell.handleOneReq(req) { break }
        }
}

func (cell *Cell) handleOneReq (req CellReq) (keepRunning bool) {
        switch req.Kind() {
        case cellKindReqDying:
                // the socket has closed, so stop accepting requests.
                return false
        }

        return true
}
