package eigenda

import (
	"context"

	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

type EigenDA struct {
	Config
}

// Init ... DA interface conformation
func (m EigenDA) Init() error {
	return nil
}

// GetSequence ...
func (m EigenDA) GetSequence(ctx context.Context, batchHashes []common.Hash, dataAvailabilityMessage []byte) ([][]byte, error) {
	log.Debug("Getting sequence from EigenDA", "batches", len(batchHashes))
	daClient := NewDAClient(m.RPC)

	b, err := daClient.GetInput(ctx, dataAvailabilityMessage)
	if err != nil {
		return nil, err
	}

	// rlp decode to 2d byte array
	var batches [][]byte
	err = rlp.DecodeBytes(b, &batches)
	if err != nil {
		return nil, err
	}

	// TODO: verify the hashes against fetched batches

	return batches, nil
}

// PostSequence ...
func (m EigenDA) PostSequence(ctx context.Context, batchesData [][]byte) ([]byte, error) {
	log.Debug("Sending sequence to EigenDA", "batches", len(batchesData))
	daClient := NewDAClient(m.RPC)
	// rlp encode 2d byte array

	b, err := rlp.EncodeToBytes(batchesData)
	if err != nil {
		return nil, err
	}

	return daClient.SetInput(ctx, b)
}
