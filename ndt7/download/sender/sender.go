// Package sender implements the download sender.
package sender

import (
	"context"
	"github.com/m-lab/ndt-server/ndt7/log"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/ndt-server/ndt7/closer"
	"github.com/m-lab/ndt-server/ndt7/measurer"
	ndt7metrics "github.com/m-lab/ndt-server/ndt7/metrics"
	"github.com/m-lab/ndt-server/ndt7/model"
	"github.com/m-lab/ndt-server/ndt7/ping"
	"github.com/m-lab/ndt-server/ndt7/spec"
)

func makePreparedMessage(size int) (*websocket.PreparedMessage, error) {
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return nil, err
	}
	return websocket.NewPreparedMessage(websocket.BinaryMessage, data)
}

// Start sends binary messages (bulk download) and measurement messages (status
// messages) to the client conn. Each measurement message will also be saved to
// data.
//
// Liveness guarantee: the sender will not be stuck sending for more than the
// MaxRuntime of the subtest. This is enforced by setting the write deadline to
// Time.Now() + MaxRuntime.
func Start(ctx context.Context, conn *websocket.Conn, data *model.ArchivalData, MaxScaledMsgSize int64, AveragePoissonSamplingInterval int64, testMetadata *model.VpimTestMetadata) error {
	log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Debug("sender: start")
	proto := ndt7metrics.ConnLabel(conn)

	// Start collecting connection measurements. Measurements will be sent to
	// src until DefaultRuntime, when the src channel is closed.
	mr := measurer.New(conn, data.UUID, AveragePoissonSamplingInterval, testMetadata, spec.SubtestDownload)
	src := mr.Start(ctx, spec.DefaultRuntime)
	defer log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Debug("sender: stop")
	defer mr.Stop(src)

	log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).Debug("sender: generating random buffer")
	bulkMessageSize := 1 << 13
	preparedMessage, err := makePreparedMessage(bulkMessageSize)
	if err != nil {
		log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Warn("sender: makePreparedMessage failed")
		ndt7metrics.ClientSenderErrors.WithLabelValues(
			proto, string(spec.SubtestDownload), "make-prepared-message").Inc()
		return err
	}
	deadline := time.Now().Add(spec.MaxRuntime)
	err = conn.SetWriteDeadline(deadline) // Liveness!
	if err != nil {
		log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Warn("sender: conn.SetWriteDeadline failed")
		ndt7metrics.ClientSenderErrors.WithLabelValues(
			proto, string(spec.SubtestDownload), "set-write-deadline").Inc()
		return err
	}

	// Record measurement start time, and prepare recording of the endtime on return.
	data.StartTime = time.Now().UTC()
	defer func() {
		data.EndTime = time.Now().UTC()
	}()
	var totalSent int64
	for {
		select {
		case m, ok := <-src:
			if !ok { // This means that the measurer has terminated
				closer.StartClosing(conn, testMetadata)
				ndt7metrics.ClientSenderErrors.WithLabelValues(
					proto, string(spec.SubtestDownload), "measurer-closed").Inc()
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Warn("sender: measurer-closed")
				return nil
			}
			if err := conn.WriteJSON(m); err != nil {
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Error("sender: conn.WriteJSON failed")
				ndt7metrics.ClientSenderErrors.WithLabelValues(
					proto, string(spec.SubtestDownload), "write-json").Inc()
				return err
			}
			// Only save measurements sent to the client.
			data.ServerMeasurements = append(data.ServerMeasurements, m)
			if err := ping.SendTicks(conn, deadline); err != nil {
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Error("sender: ping.SendTicks failed")
				ndt7metrics.ClientSenderErrors.WithLabelValues(
					proto, string(spec.SubtestDownload), "ping-send-ticks").Inc()
				return err
			}
		default:
			if err := conn.WritePreparedMessage(preparedMessage); err != nil {
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Error(
					"sender: conn.WritePreparedMessage failed")
				ndt7metrics.ClientSenderErrors.WithLabelValues(
					proto, string(spec.SubtestDownload), "write-prepared-message").Inc()
				return err
			}
			// The following block of code implements the scaling of message size
			// as recommended in the spec's appendix. We're not accounting for the
			// size of JSON messages because that is small compared to the bulk
			// message size. The net effect is slightly slowing down the scaling,
			// but this is currently fine. We need to gather data from large
			// scale deployments of this algorithm anyway, so there's no point
			// in engaging in fine grained calibration before knowing.
			totalSent += int64(bulkMessageSize)
			if int64(bulkMessageSize) >= MaxScaledMsgSize {
				continue // No further scaling is required
			}
			if int64(bulkMessageSize) > totalSent/spec.ScalingFraction {
				continue // message size still too big compared to sent data
			}
			bulkMessageSize *= 2
			preparedMessage, err = makePreparedMessage(bulkMessageSize)
			if err != nil {
				log.LogEntryWithTestMetadataAndSubtestKind(testMetadata, spec.SubtestDownload).WithError(err).Error("sender: makePreparedMessage failed")
				ndt7metrics.ClientSenderErrors.WithLabelValues(
					proto, string(spec.SubtestDownload), "make-prepared-message").Inc()
				return err
			}
		}
	}
}
