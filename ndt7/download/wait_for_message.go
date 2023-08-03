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

	receiverctx, cancel := context.WithTimeout(ctx, spec.WaitForMessageTimeout)
	defer cancel()

	currentChannel := make(chan string, 1)

	go func() {

		defer func() {
			if err := recover(); err != nil {
				logging.Logger.Warn("wait_for_message: panic occurred")
			}
		}()

		for receiverctx.Err() == nil { // Liveness!
			mtype, r, err := conn.NextReader()
			if err != nil {
				continue
			}

			if mtype != websocket.TextMessage {
				logging.Logger.Warn("wait_for_message: got non-Text message")
				continue
			}

			mdata, err := ioutil.ReadAll(r)
			if err != nil {
				logging.Logger.WithError(err).Warn("wait_for_message: reading TextMessage failed")
				continue
			}

			str := string(mdata[:])
			logging.Logger.Debug("wait_for_message: read message: " + str)

			if str != "" && str == "ready" {
				logging.Logger.Debug("wait_for_message: read ready message, sending response")
				msg := []byte("ready_response")
				if err = conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					logging.Logger.WithError(err).Warn("wait_for_message: sending ready_response failed")
					return
				}
				continue
			}

			if str != "" && str == "start" {
				logging.Logger.Debug("wait_for_message: read start message")
				currentChannel <- "wait_for_message: finished waiting for messages"
				return
			}
		}
	}()

	select {
	case res := <-currentChannel:
		logging.Logger.Debug("wait_for_message: " + res)
	case <-time.After(spec.WaitForMessageTimeout):
		logging.Logger.Warn("wait_for_message: waiting for message timed out")
	}
}
