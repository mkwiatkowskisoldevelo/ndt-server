package log

import (
	log2 "github.com/apex/log"
	"github.com/m-lab/ndt-server/logging"
	"github.com/m-lab/ndt-server/ndt7/model"
)

func LogEntryWithTestMetadata(testMetadata *model.VpimTestMetadata) *log2.Entry {
	return logging.Logger.WithField("vpimTestUUID", testMetadata.VpimTestUUID).WithField("vpimThreadIndex", testMetadata.VpimTestThreadIndex)
}
