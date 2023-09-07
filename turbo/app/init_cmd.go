package app

import (
	"encoding/json"
	"os"

	"github.com/ledgerwatch/log/v3"
	"github.com/urfave/cli/v2"

	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/node/nodecfg"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/turbo/debug"
	turboNode "github.com/ledgerwatch/erigon/turbo/node"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/firehose"
	"github.com/ledgerwatch/erigon/node"
)

var initCommand = cli.Command{
	Action:    MigrateFlags(initGenesis),
	Name:      "init",
	Usage:     "Bootstrap and initialize a new genesis block",
	ArgsUsage: "<genesisPath>",
	Flags: []cli.Flag{
		&utils.DataDirFlag,
	},
	//Category: "BLOCKCHAIN COMMANDS",
	Description: `
The init command initializes a new genesis block and definition for the network.
This is a destructive action and changes the network in which you will be
participating.

It expects the genesis file as argument.`,
}

// initGenesis will initialise the given JSON format genesis file and writes it as
// the zero'd block (i.e. genesis) or will fail hard if it can't succeed.
func initGenesis(ctx *cli.Context) error {
	var logger log.Logger
	var err error
	var nodeCfg *nodecfg.Config
	var ethCfg *ethconfig.Config
	if logger, err = debug.Setup(ctx, true /* rootLogger */, func(logger log.Logger) *types.Genesis {
		nodeCfg = turboNode.NewNodConfigUrfave(ctx, logger)
		ethCfg = turboNode.NewEthConfigUrfave(ctx, nodeCfg, logger)
		return ethCfg.Genesis
	}); err != nil {
		return err
	}

	firehose.MaybeSyncContext().InitVersion(
		params.VersionWithCommit(params.GitCommit),
		params.FirehoseVersion(),
		params.Variant,
	)

	// Make sure we have a valid genesis JSON
	genesisPath := ctx.Args().First()
	if len(genesisPath) == 0 {
		utils.Fatalf("Must supply path to genesis JSON file")
	}

	file, err := os.Open(genesisPath)
	if err != nil {
		utils.Fatalf("Failed to read genesis file: %v", err)
	}
	defer file.Close()

	genesis := new(types.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	// setting the firehose genesis to the decoded genesis
	firehose.GenesisConfig = genesis

	// Open and initialise both full and light databases
	stack := MakeConfigNodeDefault(ctx, logger)
	defer stack.Close()

	chaindb, err := node.OpenDatabase(stack.Config(), kv.ChainDB, logger)
	if err != nil {
		utils.Fatalf("Failed to open database: %v", err)
	}
	_, hash, err := core.CommitGenesisBlock(chaindb, genesis, "", logger)
	if err != nil {
		utils.Fatalf("Failed to write genesis block: %v", err)
	}
	chaindb.Close()
	logger.Info("Successfully wrote genesis state", "hash", hash.Hash())
	return nil
}
