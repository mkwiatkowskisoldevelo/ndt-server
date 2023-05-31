// Package download implements the ndt7/server downloader.
package download

import (
	"context"
	"io/ioutil"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/ndt-server/logging"
	"github.com/m-lab/ndt-server/ndt7/spec"
)

// This function suppose to wait for a message from the client side.
func WaitForMessage(ctx context.Context, conn *websocket.Conn, MaxMsgSize int64) {
	logging.Logger.Debug("wait_for_message: start")
	defer logging.Logger.Debug("wait_for_message: stop")
	conn.SetReadLimit(MaxMsgSize)
	receiverctx, cancel := context.WithTimeout(ctx, spec.MaxRuntime)
	defer cancel()
	err := conn.SetReadDeadline(time.Now().Add(spec.MaxRuntime)) // Liveness!
	if err != nil {
		logging.Logger.WithError(err).Warn("wait_for_message: conn.SetReadDeadline failed")
		return
	}

	for receiverctx.Err() == nil { // Liveness!
		// By getting a Reader here we avoid allocating memory for the message
		// when the message type is not websocket.TextMessage.
		mtype, r, err := conn.NextReader()
		if err != nil {
			continue
		}
		if mtype != websocket.TextMessage {
			logging.Logger.Warn("wait_for_message: got non-Text message")
			continue
		}
		// Identified TextMessage, reading it...
		mdata, err := ioutil.ReadAll(r)
		if err != nil {
			logging.Logger.WithError(err).Warn("wait_for_message: reading TextMessage failed")
			continue
		}
		str := string(mdata[:])
		logging.Logger.Debug("wait_for_message: read message: " + str)
		if str != "" && str == "ready" {
			logging.Logger.Debug("wait_for_message: read thread ready message")
			return
		}
	}
}
