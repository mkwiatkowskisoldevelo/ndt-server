// Package measurer collects metrics from a socket connection
// and returns them for consumption.
package measurer

import (
	"context"
	"encoding/json"
	"github.com/m-lab/ndt-server/ndt7/log"
	"github.com/m-lab/ndt-server/ndt7/spec"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/ndt-server/ndt7/model"
	"github.com/m-lab/ndt-server/netx"
)

var (
	BBREnabled = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ndt7_measurer_bbr_enabled_total",
			Help: "A counter of every attempt to enable bbr.",
		},
		[]string{"status", "error"},
	)
)

// Measurer performs measurements
type Measurer struct {
	conn                       *websocket.Conn
	uuid                       string
	ticker                     *memoryless.Ticker
	avgPoissonSamplingInterval int64
	testMetadata               *model.VpimTestMetadata
	subtestKind                *spec.SubtestKind
}

// New creates a new measurer instance
func New(conn *websocket.Conn, UUID string, avgPoissonSamplingInterval int64, testMetadata *model.VpimTestMetadata, subtestKind spec.SubtestKind) *Measurer {
	return &Measurer{
		conn:                       conn,
		uuid:                       UUID,
		avgPoissonSamplingInterval: avgPoissonSamplingInterval,
		testMetadata:               testMetadata,
		subtestKind:                &subtestKind,
	}
}

func (m *Measurer) getSocketAndPossiblyEnableBBR() (netx.ConnInfo, error) {
	ci := netx.ToConnInfo(m.conn.UnderlyingConn())
	err := ci.EnableBBR()
	success := "true"
	errstr := ""
	if err != nil {
		success = "false"
		errstr = err.Error()
		uuid, _ := ci.GetUUID() // to log error with uuid.
		log.LogEntryWithTestMetadata(m.testMetadata).WithError(err).Warn("Cannot enable BBR: " + uuid)
		// FALLTHROUGH
	}
	BBREnabled.WithLabelValues(success, errstr).Inc()
	return ci, nil
}

func (m *Measurer) measure(measurement *model.Measurement, ci netx.ConnInfo, date time.Time, elapsed time.Duration) {
	// Implementation note: we always want to sample BBR before TCPInfo so we
	// will know from TCPInfo if the connection has been closed.
	t := int64(elapsed / time.Microsecond)
	bbrinfo, tcpInfo, err := ci.ReadInfo()
	if err == nil {
		measurement.BBRInfo = &model.BBRInfo{
			BBRInfo:     bbrinfo,
			ElapsedTime: t,
			Date:        date,
		}
		measurement.TCPInfo = &model.TCPInfo{
			LinuxTCPInfo: tcpInfo,
			ElapsedTime:  t,
			Date:         date,
		}
	} else {
		log.LogEntryWithTestMetadata(m.testMetadata).WithError(err).Warn("measurer: measurer error")
	}
}

func (m *Measurer) loop(ctx context.Context, timeout time.Duration, dst chan<- model.Measurement) {
	log.LogEntryWithTestMetadataAndSubtestKind(m.testMetadata, *m.subtestKind).Debug("measurer: start")
	defer log.LogEntryWithTestMetadataAndSubtestKind(m.testMetadata, *m.subtestKind).Debug("measurer: stop")
	defer close(dst)
	measurerctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ci, err := m.getSocketAndPossiblyEnableBBR()
	if err != nil {
		log.LogEntryWithTestMetadataAndSubtestKind(m.testMetadata, *m.subtestKind).Warn("getSocketAndPossiblyEnableBBR failed")
		return
	}
	start := time.Now()
	connectionInfo := &model.ConnectionInfo{
		Client: m.conn.RemoteAddr().String(),
		Server: m.conn.LocalAddr().String(),
		UUID:   m.uuid,
	}
	// Implementation note: the ticker will close its output channel
	// after the controlling context is expired.
	ticker, err := memoryless.NewTicker(measurerctx, memoryless.Config{
		Min:      time.Duration(float64(m.avgPoissonSamplingInterval)*0.1) * time.Millisecond,
		Expected: time.Duration(m.avgPoissonSamplingInterval) * time.Millisecond,
		Max:      time.Duration(float64(m.avgPoissonSamplingInterval)*2.5) * time.Millisecond,
	})
	if err != nil {
		log.LogEntryWithTestMetadataAndSubtestKind(m.testMetadata, *m.subtestKind).Warn("memoryless.NewTicker failed")
		return
	}
	m.ticker = ticker
	for now := range ticker.C {
		var measurement model.Measurement
		m.measure(&measurement, ci, now, now.Sub(start))
		measurement.ConnectionInfo = connectionInfo
		measurementJson, _ := json.Marshal(measurement)
		log.LogEntryWithTestMetadataAndSubtestKind(m.testMetadata, *m.subtestKind).Debug("measurement: " + string(measurementJson))
		dst <- measurement // Liveness: this is blocking
	}
}

// Start runs the measurement loop in a background goroutine and emits
// the measurements on the returned channel.
//
// Liveness guarantee: the measurer will always terminate after
// the given timeout, provided that the consumer continues reading from the
// returned channel. Measurer may be stopped early by canceling ctx, or by
// calling Stop.
// kind is only used for debug logging purposes
func (m *Measurer) Start(ctx context.Context, timeout time.Duration) <-chan model.Measurement {
	dst := make(chan model.Measurement)
	go m.loop(ctx, timeout, dst)
	return dst
}

// Stop ends the measurements and drains the measurement channel. Stop
// guarantees that the measurement goroutine completes by draining the
// measurement channel. Users that call Start should also call Stop.
func (m *Measurer) Stop(src <-chan model.Measurement) {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	for range src {
		// make sure we drain the channel, so the measurement loop can exit.
	}
}
