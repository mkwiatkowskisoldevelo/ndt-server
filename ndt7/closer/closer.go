// Package closer implements the WebSocket closer.
package closer

import (
	"github.com/m-lab/ndt-server/ndt7/log"
	"github.com/m-lab/ndt-server/ndt7/model"
	"time"

	"github.com/gorilla/websocket"
)

// StartClosing will start closing the websocket connection.
func StartClosing(conn *websocket.Conn, testMetadata *model.VpimTestMetadata) {
	msg := websocket.FormatCloseMessage(
		websocket.CloseNormalClosure, "Done sending")
	d := time.Now().Add(time.Second) // Liveness!
	err := conn.WriteControl(websocket.CloseMessage, msg, d)
	if err != nil {
		log.LogEntryWithTestMetadata(testMetadata).WithError(err).Warn("sender: conn.WriteControl failed")
		return
	}
	log.LogEntryWithTestMetadata(testMetadata).Debug("sender: sending Close message")
}
