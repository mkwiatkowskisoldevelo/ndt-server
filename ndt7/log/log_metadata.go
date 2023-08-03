package log

import (
	log2 "github.com/apex/log"
	"github.com/m-lab/ndt-server/logging"
	"github.com/m-lab/ndt-server/ndt7/model"
	"github.com/m-lab/ndt-server/ndt7/spec"
)

func LogEntryWithTestMetadata(testMetadata *model.VpimTestMetadata) *log2.Entry {
	return logging.Logger.WithField("RemoteAddr", testMetadata.RemoteAddr).WithField("vpimTestUUID", testMetadata.VpimTestUUID).WithField("vpimThreadIndex", testMetadata.VpimTestThreadIndex)
}

func LogEntryWithTestMetadataAndSubtestKind(testMetadata *model.VpimTestMetadata, kind spec.SubtestKind) *log2.Entry {
	return LogEntryWithTestMetadata(testMetadata).WithField("subtestKind", kind)
}
