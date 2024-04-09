package misc

import (
	"github.com/ledgerwatch/log/v3"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/consensus"
	"github.com/ledgerwatch/erigon/core/tracing"
	"github.com/ledgerwatch/erigon/params"
)

func ApplyBeaconRootEip4788(parentBeaconBlockRoot *libcommon.Hash, syscall consensus.SystemCall, eLogger *tracing.Hooks) {
	if eLogger != nil {
		if eLogger.OnBeaconBlockRootStart != nil {
			eLogger.OnBeaconBlockRootStart(*parentBeaconBlockRoot)
		}
		if eLogger.OnBeaconBlockRootEnd != nil {
			defer func() {
				eLogger.OnBeaconBlockRootEnd()
			}()
		}
	}

	_, err := syscall(params.BeaconRootsAddress, parentBeaconBlockRoot.Bytes())
	if err != nil {
		log.Warn("Failed to call beacon roots contract", "err", err)
	}
}
