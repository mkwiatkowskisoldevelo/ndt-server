// Package download implements the ndt7/server downloader.
package download

import (
	"context"
	"github.com/m-lab/ndt-server/ndt7/log"
	"github.com/m-lab/ndt-server/ndt7/model"
	"io/ioutil"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/ndt-server/ndt7/spec"
)

// This function suppose to wait for a message from the client side.
func WaitForMessage(ctx context.Context, conn *websocket.Conn, MaxMsgSize int64, testMetadata *model.VpimTestMetadata) {
	log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Debug("wait_for_message: start")
	defer log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Debug("wait_for_message: stop")
	conn.SetReadLimit(MaxMsgSize)

	receiverctx, cancel := context.WithTimeout(ctx, spec.WaitForMessageTimeout)
	defer cancel()

	currentChannel := make(chan string, 1)
	go func() {

		defer func() {
			if err := recover(); err != nil {
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Warn("wait_for_message: panic occurred")
			}
		}()

		for receiverctx.Err() == nil { // Liveness!
			mtype, r, err := conn.NextReader()
			if err != nil {
				continue
			}
			if mtype != websocket.TextMessage {
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Warn("wait_for_message: got non-Text message")
				continue
			}

			mdata, err := ioutil.ReadAll(r)
			if err != nil {
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Warn("wait_for_message: reading TextMessage failed")
				continue
			}

			str := string(mdata[:])
			log.LogEntryWithTestMetadata(testMetadata).Debug("wait_for_message: read message: " + str)

			if str != "" && str == "ready" {
				log.LogEntryWithTestMetadata(testMetadata).Debug("wait_for_message: read ready message, sending response")
				msg := []byte("ready_response")
				if err = conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Warn("wait_for_message: sending ready_response failed")
					return
				}
				continue
			}

			if str != "" && str == "start" {
				log.LogEntryWithTestMetadata(testMetadata).Debug("wait_for_message: read start message")
				currentChannel <- "wait_for_message: finished waiting for messages"
				return
			}
		}
	}()

	select {
	case res := <-currentChannel:
		log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Debug("wait_for_message: " + res)
	case <-time.After(spec.WaitForMessageTimeout):
		log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Warn("wait_for_message: waiting for message timed out")
	}
}
