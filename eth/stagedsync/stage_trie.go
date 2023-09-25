package stagedsync

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"sync/atomic"

	"github.com/ledgerwatch/log/v3"

	"github.com/ledgerwatch/erigon-lib/kv/rawdbv3"
	"github.com/ledgerwatch/erigon/common/math"
	"github.com/ledgerwatch/erigon/core/state/temporal"
	"github.com/ledgerwatch/erigon/turbo/services"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/etl"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/state"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/turbo/trie"
)

func collectAndComputeCommitment(ctx context.Context, tx kv.RwTx, cfg TrieCfg) ([]byte, error) {
	agg, ac := tx.(*temporal.Tx).Agg(), tx.(*temporal.Tx).AggCtx()

	domains := agg.SharedDomains(ac)
	defer agg.CloseSharedDomains()

	acc := domains.Account.MakeContext()
	ccc := domains.Code.MakeContext()
	stc := domains.Storage.MakeContext()

	defer acc.Close()
	defer ccc.Close()
	defer stc.Close()

	_, _, _, err := domains.SeekCommitment(0, math.MaxUint64)
	if err != nil {
		return nil, err
	}

	logger := log.New("stage", "patricia_trie", "block", domains.BlockNum())
	logger.Info("Collecting account keys")
	collector := etl.NewCollector("collect_keys", cfg.tmpDir, etl.NewSortableBuffer(etl.BufferOptimalSize/2), logger)
	defer collector.Close()

	var totalKeys atomic.Uint64
	for _, dc := range []*state.DomainContext{acc, ccc, stc} {
		logger.Info("Collecting keys")
		err := dc.IteratePrefix(tx, nil, func(k []byte, _ []byte) error {
			if err := collector.Collect(k, nil); err != nil {
				return err
			}
			totalKeys.Add(1)
			return ctx.Err()
		})
		if err != nil {
			return nil, err
		}
	}

	var (
		batchSize = uint64(10_000_000)
		processed atomic.Uint64
	)

	loadKeys := func(k, v []byte, table etl.CurrentTableReader, next etl.LoadNextFunc) error {
		if domains.Commitment.Size() >= batchSize {
			rh, err := domains.Commit(true, false)
			if err != nil {
				return err
			}
			logger.Info("Committing batch",
				"processed", fmt.Sprintf("%d/%d (%.2f%%)",
					processed.Load(), totalKeys.Load(), 100*(float64(totalKeys.Load())/float64(processed.Load()))),
				"intermediate root", rh)
		}
		processed.Add(1)
		domains.Commitment.TouchPlainKey(k, nil, nil)

		return nil
	}
	err = collector.Load(nil, "", loadKeys, etl.TransformArgs{Quit: ctx.Done()})
	if err != nil {
		return nil, err
	}
	collector.Close()

	rh, err := domains.Commit(true, false)
	if err != nil {
		return nil, err
	}
	logger.Info("Commitment has been reevaluated", "tx", domains.TxNum(), "root", hex.EncodeToString(rh), "processed", processed.Load(), "total", totalKeys.Load())

	if err := cfg.agg.Flush(ctx, tx); err != nil {
		return nil, err
	}

	return rh, nil
}

func countBlockByTxnum(ctx context.Context, tx kv.Tx, txnum uint64, blockReader services.FullBlockReader) (blockNum uint64, notInTheMiddle bool, err error) {
	var txCounter uint64 = 0
	var ft, lt uint64

	for i := uint64(0); i < math.MaxUint64; i++ {
		if i%1000000 == 0 {
			fmt.Printf("\r [%s] Counting block for tx %d: cur block %d cur tx %d\n", "restoreCommit", txnum, i, txCounter)
		}

		h, err := blockReader.HeaderByNumber(ctx, tx, uint64(i))
		if err != nil {
			return 0, false, err
		}

		ft = txCounter
		txCounter++
		b, err := blockReader.BodyWithTransactions(ctx, tx, h.Hash(), uint64(i))
		if err != nil {
			return 0, false, err
		}
		txCounter += uint64(len(b.Transactions))
		txCounter++
		blockNum = i
		lt = txCounter

		if txCounter >= txnum {
			return blockNum, ft == txnum || lt == txnum, nil
		}
	}
	return 0, false, fmt.Errorf("block not found")

}

func SpawnPatriciaTrieStage(tx kv.RwTx, cfg TrieCfg, ctx context.Context, logger log.Logger) (libcommon.Hash, error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		var err error
		tx, err = cfg.db.BeginRw(context.Background())
		if err != nil {
			return trie.EmptyRoot, err
		}
		defer tx.Rollback()
	}

	var foundHash bool
	agg := tx.(*temporal.Tx).Agg()
	toTxNum := agg.EndTxNumNoCommitment()
	ok, blockNum, err := rawdbv3.TxNums.FindBlockNum(tx, toTxNum)
	if err != nil {
		return libcommon.Hash{}, err
	}
	if !ok {
		blockNum, foundHash, err = countBlockByTxnum(ctx, tx, toTxNum, cfg.blockReader)
		if err != nil {
			return libcommon.Hash{}, err
		}
	} else {
		firstTxInBlock, err := rawdbv3.TxNums.Min(tx, blockNum)
		if err != nil {
			return libcommon.Hash{}, fmt.Errorf("failed to find first txNum in block %d : %w", blockNum, err)
		}
		lastTxInBlock, err := rawdbv3.TxNums.Max(tx, blockNum)
		if err != nil {
			return libcommon.Hash{}, fmt.Errorf("failed to find last txNum in block %d : %w", blockNum, err)
		}
		if firstTxInBlock == toTxNum || lastTxInBlock == toTxNum {
			foundHash = true // state is in the beginning or end of block
		}
	}

	var expectedRootHash libcommon.Hash
	var headerHash libcommon.Hash
	var syncHeadHeader *types.Header
	if cfg.checkRoot {
		syncHeadHeader, err = cfg.blockReader.HeaderByNumber(ctx, tx, blockNum)
		if err != nil {
			return trie.EmptyRoot, err
		}
		if syncHeadHeader == nil {
			return trie.EmptyRoot, fmt.Errorf("no header found with number %d", blockNum)
		}
		expectedRootHash = syncHeadHeader.Root
		headerHash = syncHeadHeader.Hash()
	}

	rh, err := collectAndComputeCommitment(ctx, tx, cfg)
	if err != nil {
		return trie.EmptyRoot, err
	}
	//if !foundHash { // tx could be in the middle of block so no header match will be found
	//	return trie.EmptyRoot, fmt.Errorf("no header found with root %x", rh)
	//}

	if (foundHash || cfg.checkRoot) && !bytes.Equal(rh, expectedRootHash[:]) {
		logger.Error(fmt.Sprintf("[RebuildCommitment] Wrong trie root of block %d: %x, expected (from header): %x. Block hash: %x", blockNum, rh, expectedRootHash, headerHash))
		if cfg.badBlockHalt {
			return trie.EmptyRoot, fmt.Errorf("wrong trie root")
		}
	} else {
		logger.Info(fmt.Sprintf("[RebuildCommitment] Trie root of block %d txNum %d: %x. Could not verify with block hash because txnum of state is in the middle of the block.", blockNum, rh, toTxNum))
	}

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return trie.EmptyRoot, err
		}
	}
	return libcommon.BytesToHash(rh), err
}
