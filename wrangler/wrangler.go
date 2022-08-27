package wrangler

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hlhv/fsock"
	"github.com/hlhv/hlhv-queen/cells"
	"github.com/hlhv/hlhv-queen/conf"
	"github.com/hlhv/protocol"
	"github.com/hlhv/scribe"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

var port string
var cert tls.Certificate
var config tls.Config
var listening bool
var stopNotify chan int
var server net.Listener

var cellStore struct {
	lookup map[string]*cells.Cell
	mutex  sync.Mutex
}

/* Arm initializes the cell wrangler, loading the certificate and initializing
 * maps.
 */
func Arm() (err error) {
	port = strconv.Itoa(conf.GetPortHlhv())
	scribe.PrintProgress(
		scribe.LogLevelNormal,
		"arming cell wrangler on port", port)

	keyPath := conf.GetKeyPath()
	certPath := conf.GetCertPath()
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return errors.New(
			"certificate is not present or inaccessible")
	}

	config = tls.Config{Certificates: []tls.Certificate{cert}}

	cellStore.lookup = make(map[string]*cells.Cell)

	return nil
}

/* Fire is suppsoed to be run in a separate goroutine, and handles incoming
 * requests on the hlhv port. It decides what those connections are and creates
 * new Cells and Bands out of them. This function will only run after the
 * wrangler has been Arm()'d.
 */
func Fire() {
	scribe.PrintInfo(
		scribe.LogLevelDebug,
		"wrangler listening")
	listening = true
	defer func() {
		listening = false
		scribe.PrintInfo(
			scribe.LogLevelDebug,
			"wrangler no longer listening")
	}()

	var err error
	server, err = tls.Listen("tcp", ":"+port, &config)
	if err != nil {
		scribe.PrintFatal(scribe.LogLevelError, err.Error())
		return
	}

	go Garden()

	for {
		conn, err := server.Accept()

		// if we are stopping, exit cleanly
		if stopNotify != nil {
			stopNotify <- 0
			return
		}

		if err != nil {
			scribe.PrintFatal(
				scribe.LogLevelError,
				"wrangler accept:", err)
			return
			// TODO: maybe re-create the listener?
		}

		scribe.PrintConnect(scribe.LogLevelNormal, "new connection")
		err = handleConn(conn)

		if err != nil {
			scribe.PrintError(
				scribe.LogLevelError,
				"wrangler accept:", err)
			continue
		}
	}
}

/* Close stops the cell wrangler from accepting new cell connections. The fire
 * function should not be called again after calling this.
 */
func Close() {
	if !listening {
		return
	}

	scribe.PrintProgress(scribe.LogLevelNormal, "stopping cell wrangler")
	stopNotify = make(chan int)
	server.Close()
	<-stopNotify
	scribe.PrintDone(scribe.LogLevelNormal, "stopped cell wrangler")
}

/* handleConn takes in an incoming connection, and decides what to do with it.
 * Currently, it can accept new cells and bands.
 */
func handleConn(conn net.Conn) (err error) {
	reader := fsock.NewReader(conn)
	writer := fsock.NewWriter(conn)

	scribe.PrintProgress(
		scribe.LogLevelDebug,
		"waiting for logon")
	bumpTimeout(conn)
	kind, data, err := protocol.ReadParseFrame(reader)
	if err != nil {
		conn.Close()
		scribe.PrintDisconnect(scribe.LogLevelNormal, "kicked")
		return errors.New(fmt.Sprint(
			"error parsing login frame: ", err.Error()))
	}

	if kind != protocol.FrameKindIAm {
		conn.Close()
		scribe.PrintDisconnect(scribe.LogLevelNormal, "kicked")
		return errors.New(fmt.Sprint(
			"cell sent strange kind code: ", kind))
	}

	frame := protocol.FrameIAm{}
	err = json.Unmarshal(data, &frame)
	if err != nil {
		conn.Close()
		scribe.PrintDisconnect(scribe.LogLevelNormal, "kicked")
		return errors.New(fmt.Sprint(
			"error unmarshaling login frame: ", err.Error()))
	}

	switch frame.ConnKind {
	case protocol.ConnKindCell:
		err = handleConnCell(conn, reader, writer, frame.Key)
		if err != nil {
			conn.Close()
			scribe.PrintDisconnect(scribe.LogLevelNormal, "kicked")
			return err
		}
		scribe.PrintDone(scribe.LogLevelNormal, "accepted cell")
		break
	case protocol.ConnKindBand:
		err = handleConnBand(conn, reader, writer, frame.Uuid, frame.Key)
		if err != nil {
			conn.Close()
			scribe.PrintDisconnect(scribe.LogLevelNormal, "kicked")
			return err
		}
		scribe.PrintDone(scribe.LogLevelNormal, "accepted band")
		break
	}

	return nil
}

/* handleConnCell creates a new Cell fom a connection and adds it to the
 * wrangler's list of Cells. If something goes wrong, this function will return
 * an error. This function does not close the channel in response to an error,
 * this is the responsibility of handleConn(). This function assumes that the
 * connection wishes to become a Cell.
 */
func handleConnCell(
	leash net.Conn,
	reader *fsock.Reader,
	writer *fsock.Writer,
	key string,
) (
	err error,
) {
	bumpTimeout(leash)

	var cell *cells.Cell

	// generate a uuid and slap that hoe into the cell store
	var uuidString string
	for {
		uuid := uuid.New()
		uuidString = uuid.String()

		_, exists := cellStore.lookup[uuidString]
		if !exists {
			cell = cells.NewCell(
				leash, reader, writer,
				uuidString, cleanUpCell)
			cellStore.lookup[uuidString] = cell
			break
		}

		// if by some weird chance the uuid exists, make a new one
	}

	// inform the cell that it has been accepted, and give it the uuid
	_, err = protocol.WriteMarshalFrame(writer, &protocol.FrameAccept{
		Uuid: uuidString,
		Key:  cell.Key(),
	})
	if err != nil {
		return err
	}

	clearTimeout(leash)
	go cell.Listen()
	go cell.ListenSig()
	return nil
}

/* handleConnBand creates a new Band from a connection and adds it to the Cell
 * of the specified uuid. If something goes wrong, this function will return
 * an error. This function does not close the channel in response to an error,
 * this is the responsibility of handleConn(). This function assumes that the
 * connection wishes to become a Band.
 */
func handleConnBand(
	conn net.Conn,
	reader *fsock.Reader,
	writer *fsock.Writer,
	uuid string,
	key string,
) (
	err error,
) {
	bumpTimeout(conn)

	cell, exists := cellStore.lookup[uuid]
	if !exists {
		return errors.New(fmt.Sprint(
			"error binding band: no cell called", uuid))
	}

	band := cells.NewBand(conn, reader, writer)

	// add band to cell
	err = cell.Bind(band, key)
	if err != nil {
		return err
	}

	// inform the band that it has been accepted
	_, err = protocol.WriteMarshalFrame(writer, &protocol.FrameAccept{
		Uuid: uuid,
	})

	clearTimeout(conn)
	return err
}

/* Garden is a goroutine that prunes cells on an interval.
 */
func Garden() {
	for {
		time.Sleep(time.Duration(conf.GetGardenFreq()) * time.Second)

		// if the cell is not listening, we can safely stop gardening.
		if !listening {
			return
		}

		pruned := 0
		scribe.PrintProgress(scribe.LogLevelDebug, "pruning cell bands")
		for _, cell := range cellStore.lookup {
			pruned += cell.Prune()
		}
		scribe.PrintDone(scribe.LogLevelDebug, pruned, "bands pruned")
	}
	scribe.PrintFatal(
		scribe.LogLevelError,
		"gardener has stopped, will not attempt to run without it!")
	os.Exit(1)
}

/* This function is called by cells when their leashes close. It removes the
 * cell from the wrangler's list.
 */
func cleanUpCell(cell *cells.Cell) {
	delete(cellStore.lookup, cell.Uuid())
}

/* bumpTimeout sets the read timeout of a connection to however many seconds in
 * the future specified by conf.
 */
func bumpTimeout(conn net.Conn) {
	duration := time.Duration(conf.GetTimeout()) * time.Second
	conn.SetDeadline(time.Now().Add(duration))
}

/* clearTimeout clears the timeout for a connection.
 */
func clearTimeout(conn net.Conn) {
	conn.SetDeadline(time.Time{})
}
