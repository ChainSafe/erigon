package firehose_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erigontech/erigon/eth/tracers/live"

	"github.com/stretchr/testify/require"
)

func TestFirehosePrestate(t *testing.T) {
	testFolders := []string{
		// "./testdata/TestFirehosePrestate/keccak256_too_few_memory_bytes_get_padded",
		"./testdata/TestFirehosePrestate/keccak256_wrong_diff",
	}

	for _, folder := range testFolders {
		name := filepath.Base(folder)

		t.Run(name, func(t *testing.T) {
			tracer, err := live.NewFirehoseFromRawJSON(json.RawMessage(`{
				"applyBackwardsCompatibility": true,
				"_private": {
					"flushToTestBuffer": true
				}
			}`))
			require.NoError(t, err)

			runPrestateBlock(t, filepath.Join(folder, "prestate.json"), live.NewTracingHooksFromFirehose(tracer))
			genesisLine, blockLines, unknownLines := readTracerFirehoseLines(t, tracer)
			require.Len(t, unknownLines, 0, "Lines:\n%s", strings.Join(slicesMap(unknownLines, func(l unknwonLine) string { return "- '" + string(l) + "'" }), "\n"))
			require.NotNil(t, genesisLine)
			blockLines.assertOnlyBlockEquals(t, folder, 1)
		})
	}

}
