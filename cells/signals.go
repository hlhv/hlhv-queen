package cells

import (
        "github.com/hlhv/protocol"
        "github.com/hlhv/scribe"
)

type Sig int

const (
        SigCleaning Sig = iota
        SigNeedBand
)

func (cell *Cell) ListenSig () {
        for {
                sig := <- cell.sigQueue
                if !cell.handleSig(sig) { break }
        }
}

func (cell *Cell) handleSig (sig Sig) (run bool) {
        writer := cell.Writer

        switch sig {
        case SigCleaning:
                return false
        case SigNeedBand:
                scribe.PrintProgress (
                        scribe.LogLevelDebug,
                        "requesting new band")
                protocol.WriteMarshalFrame (writer, &protocol.FrameNeedBand {
                        Count: 1,
                })
        }

        return true
}

func (cell *Cell) SendSig (sig Sig) {
        cell.sigQueue <- sig
}
