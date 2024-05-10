package firehose_test

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/common/math"
	"github.com/ledgerwatch/erigon/consensus"
	"github.com/ledgerwatch/erigon/consensus/ethash"
	"github.com/ledgerwatch/erigon/consensus/misc"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/vm/evmtypes"
	"github.com/ledgerwatch/log/v3"

	"github.com/stretchr/testify/require"
)

func readPrestateData(t *testing.T, path string) *prestateData {
	t.Helper()

	// Call tracer test found, read if from disk
	blob, err := os.ReadFile(path)
	require.NoError(t, err)

	test := new(prestateData)
	require.NoError(t, json.Unmarshal(blob, test))

	var genesisWithTD struct {
		Genesis struct {
			TotalDifficulty *math.HexOrDecimal256 `json:"totalDifficulty"`
		} `json:"genesis"`
	}
	if err := json.Unmarshal(blob, &genesisWithTD); err == nil {
		test.TotalDifficulty = (*big.Int)(genesisWithTD.Genesis.TotalDifficulty)
	}

	return test
}

type prestateData struct {
	Genesis         *types.Genesis  `json:"genesis"`
	Context         *callContext    `json:"context"`
	Input           string          `json:"input"`
	TotalDifficulty *big.Int        `json:"-"`
	TracerConfig    json.RawMessage `json:"tracerConfig"`

	// Populated after loading
	genesisBlock *types.Block
}

// Engine implements core.ChainContext.
func (p *prestateData) Engine() consensus.Engine {
	return ethash.NewFullFaker()
}

// GetHeader implements core.ChainContext.
func (p *prestateData) GetHeader(hash common.Hash, number uint64) (*types.Header, error) {
	var err error
	var logger = log.New("test")
	if p.Genesis == nil {
		return nil, nil
	}

	if p.genesisBlock == nil {
		p.genesisBlock, _, err = core.GenesisToBlock(p.Genesis, "", logger, nil)
		if err != nil {
			return nil, err
		}
	}

	if hash == p.genesisBlock.Hash() {
		return p.genesisBlock.Header(), nil
	}

	if number == p.genesisBlock.NumberU64() {
		return p.genesisBlock.Header(), nil
	}

	return nil, nil
}

type callContext struct {
	Number     math.HexOrDecimal64   `json:"number"`
	Difficulty *math.HexOrDecimal256 `json:"difficulty"`
	Time       math.HexOrDecimal64   `json:"timestamp"`
	GasLimit   math.HexOrDecimal64   `json:"gasLimit"`
	Miner      common.Address        `json:"miner"`
	BaseFee    *math.HexOrDecimal256 `json:"baseFeePerGas"`
}

func (c *callContext) toBlockContext(genesis *types.Genesis) (evmtypes.BlockContext, error) {
	context := evmtypes.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    c.Miner,
		BlockNumber: (uint64(c.Number)),
		Time:        uint64(c.Time),
		Difficulty:  (*big.Int)(c.Difficulty),
		GasLimit:    uint64(c.GasLimit),
	}

	if genesis.Config.IsLondon(context.BlockNumber) {
		baseFee, _ := uint256.FromBig((*big.Int)(c.BaseFee))
		context.BaseFee = baseFee
	}

	if genesis.ExcessBlobGas != nil && genesis.BlobGasUsed != nil {
		var logger = log.New("test")
		genesisBlock, _, err := core.GenesisToBlock(genesis, "", logger, nil)
		if err != nil {
			return evmtypes.BlockContext{}, err
		}

		excessBlobGas := misc.CalcExcessBlobGas(genesis.Config, genesisBlock.Header())
		context.ExcessBlobGas = &excessBlobGas
	}

	return context, nil
}
