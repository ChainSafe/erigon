package vm

import (
	"fmt"

	libcommon "github.com/ledgerwatch/erigon-lib/common"

	"github.com/ledgerwatch/erigon/core/vm/evmtypes"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/firehose"
)

func NewCVM(state evmtypes.IntraBlockState, firehoseContext *firehose.Context) *CVM {
	cvm := &CVM{
		intraBlockState: state,
		firehoseContext: firehoseContext,
	}

	return cvm
}

type CVM struct {
	config          Config
	intraBlockState evmtypes.IntraBlockState
	firehoseContext *firehose.Context
}

func (cvm *CVM) Create(caller ContractRef, code []byte) ([]byte, libcommon.Address, error) {
	address := crypto.CreateAddress(caller.Address(), cvm.intraBlockState.GetNonce(caller.Address()))
	// CS TODO: check if we need to provide actual firehose context here
	cvm.intraBlockState.SetCode(address, code, cvm.firehoseContext)
	fmt.Println(">>>> Create Starknet Contract", address.Hex())
	return code, libcommon.Address{}, nil

	//TODO:: execute cairo construct
	//ret, err := cvm.run(code)
}

func (cvm *CVM) Config() Config {
	return cvm.config
}

func (cvm *CVM) IntraBlockState() evmtypes.IntraBlockState {
	return cvm.intraBlockState
}

//func (cvm *CVM) run(code []byte) ([]byte, error) {
//	// TODO:: call grpc cairo
//	return code, nil
//}
