package eigenda

import (
	"context"
	"errors"
	"fmt"

	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	EigenDAV1 = 1
)

var (
	ErrInvalidSequence = errors.New("invalid sequence fetched from EigenDA")
)

type EigenDA struct {
	Config
}

// Init ... DA interface conformation
func (m EigenDA) Init() error {
	return nil
}

// EncodeVersion ... Encodes version byte to the commit
func EncodeVersion(rawCommit []byte, version uint8) []byte {
	ver := make([]byte, EigenDAV1)
	ver[0] = version

	return append(ver, rawCommit...)
}

// DecodeVersion ... Decodes version byte from the commit
func DecodeVersion(rawCommit []byte) ([]byte, error) {
	if len(rawCommit) == 0 {
		return nil, errors.New("commit is empty")
	}

	rawCommit = rawCommit[1:]

	return rawCommit, nil

}

// GetSequence ... Fetches the sequence of batches associated with some commit from EigenDA
func (m EigenDA) GetSequence(ctx context.Context, batchHashes []common.Hash, commit []byte) ([][]byte, error) {
	log.Debug(fmt.Sprintf("Getting sequence of %d batches to EigenDA", len(batchHashes)))
	daClient := NewDAClient(m.RPC)

	// decode version
	commit, err := DecodeVersion(commit)
	if err != nil {
		return nil, err
	}

	// append 0x1 to the commit for server side encoding compatibility
	commit = append([]byte{0x1}, commit...)

	b, err := daClient.GetInput(ctx, commit)
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
	log.Debug(fmt.Sprintf("Sending sequence of %d batches to EigenDA", len(batchesData)))
	daClient := NewDAClient(m.RPC)

	// rlp encode to bytes
	b, err := rlp.EncodeToBytes(batchesData)
	if err != nil {
		return nil, err
	}

	// post the data to EigenDA
	rawCommit, err := daClient.SetInput(ctx, b)
	if err != nil {
		return nil, err
	}

	// encode version to the rawCommit for posting on-chain
	return EncodeVersion(rawCommit, EigenDAV1), nil
}
