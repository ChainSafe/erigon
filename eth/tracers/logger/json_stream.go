package logger

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"

	"github.com/holiman/uint256"
	jsoniter "github.com/json-iterator/go"
	"github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"

	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/vm"
	"github.com/ledgerwatch/erigon/core/vm/evmtypes"
)

// JsonStreamLogger is an EVM state logger and implements Tracer.
//
// JsonStreamLogger can capture state based on the given Log configuration and also keeps
// a track record of modified storage which is used in reporting snapshots of the
// contract their storage.
type JsonStreamLogger struct {
	ctx          context.Context
	cfg          LogConfig
	stream       *jsoniter.Stream
	hexEncodeBuf [128]byte
	firstCapture bool

	locations common.Hashes // For sorting
	storage   map[libcommon.Address]Storage
	logs      []StructLog
	output    []byte //nolint
	err       error  //nolint
	env       *vm.EVM
}

// NewStructLogger returns a new logger
func NewJsonStreamLogger(cfg *LogConfig, ctx context.Context, stream *jsoniter.Stream) *JsonStreamLogger {
	logger := &JsonStreamLogger{
		ctx:          ctx,
		stream:       stream,
		storage:      make(map[libcommon.Address]Storage),
		firstCapture: true,
	}
	if cfg != nil {
		logger.cfg = *cfg
	}
	return logger
}

func (l *JsonStreamLogger) CaptureTxStart(env *vm.EVM, tx types.Transaction) {
	l.env = env
}

func (l *JsonStreamLogger) CaptureTxEnd(receipt *types.Receipt, err error) {}

func (l *JsonStreamLogger) CaptureStart(from libcommon.Address, to libcommon.Address, precompile bool, create bool, input []byte, gas uint64, value *uint256.Int, code []byte) {
}

func (l *JsonStreamLogger) CaptureEnter(typ vm.OpCode, from libcommon.Address, to libcommon.Address, precompile bool, create bool, input []byte, gas uint64, value *uint256.Int, code []byte) {
}

// CaptureState logs a new structured log message and pushes it out to the environment
//
// CaptureState also tracks SLOAD/SSTORE ops to track storage change.
func (l *JsonStreamLogger) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	contract := scope.Contract
	memory := scope.Memory
	stack := scope.Stack

	select {
	case <-l.ctx.Done():
		return
	default:
	}
	// check if already accumulated the specified number of logs
	if l.cfg.Limit != 0 && l.cfg.Limit <= len(l.logs) {
		return
	}
	if !l.firstCapture {
		l.stream.WriteMore()
	} else {
		l.firstCapture = false
	}
	var outputStorage bool
	if !l.cfg.DisableStorage {
		// initialise new changed values storage container for this contract
		// if not present.
		if l.storage[contract.Address()] == nil {
			l.storage[contract.Address()] = make(Storage)
		}
		// capture SLOAD opcodes and record the read entry in the local storage
		if op == vm.SLOAD && stack.Len() >= 1 {
			var (
				address = libcommon.Hash(stack.Data[stack.Len()-1].Bytes32())
				value   uint256.Int
			)
			l.env.IntraBlockState().GetState(contract.Address(), &address, &value)
			l.storage[contract.Address()][address] = value.Bytes32()
			outputStorage = true
		}
		// capture SSTORE opcodes and record the written entry in the local storage.
		if op == vm.SSTORE && stack.Len() >= 2 {
			var (
				value   = libcommon.Hash(stack.Data[stack.Len()-2].Bytes32())
				address = libcommon.Hash(stack.Data[stack.Len()-1].Bytes32())
			)
			l.storage[contract.Address()][address] = value
			outputStorage = true
		}
	}
	// create a new snapshot of the EVM.
	l.stream.WriteObjectStart()
	l.stream.WriteObjectField("pc")
	l.stream.WriteUint64(pc)
	l.stream.WriteMore()
	l.stream.WriteObjectField("op")
	l.stream.WriteString(op.String())
	l.stream.WriteMore()
	l.stream.WriteObjectField("gas")
	l.stream.WriteUint64(gas)
	l.stream.WriteMore()
	l.stream.WriteObjectField("gasCost")
	l.stream.WriteUint64(cost)
	l.stream.WriteMore()
	l.stream.WriteObjectField("depth")
	l.stream.WriteInt(depth)
	if err != nil {
		l.stream.WriteMore()
		l.stream.WriteObjectField("error")
		l.stream.WriteObjectStart()
		l.stream.WriteObjectEnd()
		//l.stream.WriteString(err.Error())
	}
	if !l.cfg.DisableStack {
		l.stream.WriteMore()
		l.stream.WriteObjectField("stack")
		l.stream.WriteArrayStart()
		for i, stackValue := range stack.Data {
			if i > 0 {
				l.stream.WriteMore()
			}
			l.stream.WriteString(stackValue.String())
		}
		l.stream.WriteArrayEnd()
	}
	if !l.cfg.DisableMemory {
		memData := memory.Data()
		l.stream.WriteMore()
		l.stream.WriteObjectField("memory")
		l.stream.WriteArrayStart()
		for i := 0; i+32 <= len(memData); i += 32 {
			if i > 0 {
				l.stream.WriteMore()
			}
			l.stream.WriteString(string(l.hexEncodeBuf[0:hex.Encode(l.hexEncodeBuf[:], memData[i:i+32])]))
		}
		l.stream.WriteArrayEnd()
	}
	if outputStorage {
		l.stream.WriteMore()
		l.stream.WriteObjectField("storage")
		l.stream.WriteObjectStart()
		first := true
		// Sort storage by locations for easier comparison with geth
		if l.locations != nil {
			l.locations = l.locations[:0]
		}
		s := l.storage[contract.Address()]
		for loc := range s {
			l.locations = append(l.locations, loc)
		}
		sort.Sort(l.locations)
		for _, loc := range l.locations {
			value := s[loc]
			if first {
				first = false
			} else {
				l.stream.WriteMore()
			}
			l.stream.WriteObjectField(string(l.hexEncodeBuf[0:hex.Encode(l.hexEncodeBuf[:], loc[:])]))
			l.stream.WriteString(string(l.hexEncodeBuf[0:hex.Encode(l.hexEncodeBuf[:], value[:])]))
		}
		l.stream.WriteObjectEnd()
	}
	l.stream.WriteObjectEnd()
	_ = l.stream.Flush()
}

// CaptureFault implements the Tracer interface to trace an execution fault
// while running an opcode.
func (l *JsonStreamLogger) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
}

func (l *JsonStreamLogger) CaptureEnd(output []byte, gasUsed uint64, err error, reverted bool) {
}

func (l *JsonStreamLogger) OnBlockStart(b *types.Block, td *big.Int, finalized, safe *types.Header, chainConfig *chain.Config) {
}

func (l *JsonStreamLogger) OnBlockEnd(err error) {
}

func (l *JsonStreamLogger) OnGenesisBlock(b *types.Block, alloc types.GenesisAlloc) {
}

func (l *JsonStreamLogger) OnBeaconBlockRootStart(root libcommon.Hash) {}

func (l *JsonStreamLogger) OnBeaconBlockRootEnd() {}

func (l *JsonStreamLogger) CaptureKeccakPreimage(hash libcommon.Hash, data []byte) {}

func (l *JsonStreamLogger) OnGasChange(old, new uint64, reason vm.GasChangeReason) {}

func (l *JsonStreamLogger) OnBalanceChange(a libcommon.Address, prev, new *uint256.Int, reason evmtypes.BalanceChangeReason) {
}

func (l *JsonStreamLogger) OnNonceChange(a libcommon.Address, prev, new uint64) {}

func (l *JsonStreamLogger) OnCodeChange(a libcommon.Address, prevCodeHash libcommon.Hash, prev []byte, codeHash libcommon.Hash, code []byte) {
}

func (l *JsonStreamLogger) OnStorageChange(a libcommon.Address, k *libcommon.Hash, prev, new uint256.Int) {
}

func (l *JsonStreamLogger) OnLog(log *types.Log) {}

func (l *JsonStreamLogger) OnNewAccount(a libcommon.Address, reset bool) {}

func (l *JsonStreamLogger) CaptureExit(output []byte, usedGas uint64, err error, reverted bool) {
}

// GetResult returns an empty json object.
func (l *JsonStreamLogger) GetResult() (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// Stop terminates execution of the tracer at the first opportune moment.
func (l *JsonStreamLogger) Stop(err error) {
}
