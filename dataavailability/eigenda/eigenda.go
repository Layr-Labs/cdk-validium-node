package eigenda

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/Layr-Labs/eigenda/api/grpc/disperser"
	"github.com/Layr-Labs/eigenda/encoding/utils/codec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type EigenDA struct {
	Config
}

// EigenDAMessage ... message that's sent to L1 sequencing contract
// that provides a reference to the EigenDA blob so it can be looked up during state derivation
// TODO - introduce version byte to allow for future upgrades to the message format
type EigenDAMessage struct {
	BlobHeader []byte
	BlobIndex  uint32
}

// Init ... DA interface conformation
func (m EigenDA) Init() error {
	return nil
}

// GetSequence ... Reads the data from the EigenDA blob
// NOTE - The hash of each blob can be verified for inclusion in the batchHashes set
// to ensure that the data hasn't been tampered with by external DA.
// In the future, blobs can also be verified for equivalence to their respective kzg commitments
// if the associated commitments are also stored on-chain as part of the da message.
func (m EigenDA) GetSequence(ctx context.Context, batchHashes []common.Hash, dataAvailabilityMessage []byte) ([][]byte, error) {
	var msg EigenDAMessage
	if err := rlp.DecodeBytes(dataAvailabilityMessage, &msg); err != nil {
		return nil, err
	}

	config := &tls.Config{}
	credential := credentials.NewTLS(config)
	dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(credential)}
	conn, err := grpc.Dial(m.RPC, dialOptions...)
	if err != nil {
		return nil, err
	}
	daClient := disperser.NewDisperserClient(conn)

	request := &disperser.RetrieveBlobRequest{
		BatchHeaderHash: msg.BlobHeader,
		BlobIndex:       msg.BlobIndex,
	}

	reply, err := daClient.RetrieveBlob(ctx, request)
	if err != nil {
		return nil, err
	}

	// decode modulo bn254
	decodedData := codec.RemoveEmptyByteFromPaddedBytes(reply.Data)

	// rlp decode to batches
	var batches [][]byte
	if err := rlp.DecodeBytes(decodedData, &batches); err != nil {
		return nil, err
	}

	return batches, nil
}

func (m EigenDA) PostSequence(ctx context.Context, batchesData [][]byte) ([]byte, error) {
	log.Debug("Sending sequence to EigenDA", "batches", len(batchesData))
	config := &tls.Config{}
	credential := credentials.NewTLS(config)
	dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(credential)}
	conn, err := grpc.Dial(m.RPC, dialOptions...)
	if err != nil {
		return nil, err
	}

	daClient := disperser.NewDisperserClient(conn)

	// Map N batches to 1 one blob
	encodedBytes, err := rlp.EncodeToBytes(batchesData)
	if err != nil {
		return nil, err
	}

	// encode modulo bn254
	encodedTxData := codec.ConvertByPaddingEmptyByte(encodedBytes)

	disperseReq := &disperser.DisperseBlobRequest{
		Data: encodedTxData,
	}

	disperseRes, err := daClient.DisperseBlob(ctx, disperseReq)

	if err != nil || disperseRes == nil {
		log.Error("Unable to disperse blob to EigenDA, aborting", "err", err)
		return nil, err
	}

	if disperseRes.Result == disperser.BlobStatus_UNKNOWN ||
		disperseRes.Result == disperser.BlobStatus_FAILED {
		log.Error("Unable to disperse blob to EigenDA, aborting", "err", err)
		return nil, fmt.Errorf("reply status is %d", disperseRes.Result)
	}

	base64RequestID := base64.StdEncoding.EncodeToString(disperseRes.RequestId)

	var statusRes *disperser.BlobStatusReply
	timeoutTime := time.Now().Add(time.Duration(m.StatusQueryTimeoutSeconds) * time.Second)
	// Wait before first status check
	time.Sleep(time.Duration(m.StatusQueryRetryIntervalSeconds) * time.Second)

	// Poll for blob confirmation until timeout
	for time.Now().Before(timeoutTime) {
		statusRes, err = daClient.GetBlobStatus(ctx, &disperser.BlobStatusRequest{
			RequestId: disperseRes.RequestId,
		})
		if err != nil {

		} else if statusRes.Status == disperser.BlobStatus_CONFIRMED || statusRes.Status == disperser.BlobStatus_FINALIZED {
			batchHeaderHashHex := fmt.Sprintf("0x%s", hex.EncodeToString(statusRes.Info.BlobVerificationProof.BatchMetadata.BatchHeaderHash))
			log.Info("EigenDA blob dispersed successfully", "requestID", base64RequestID, "batchHeaderHash", batchHeaderHashHex, "blobIndex", statusRes.Info.BlobVerificationProof.BlobIndex)

			msg, err := rlp.EncodeToBytes(EigenDAMessage{
				BlobHeader: statusRes.Info.BlobVerificationProof.BatchMetadata.BatchHeaderHash,
				BlobIndex:  statusRes.Info.BlobVerificationProof.BlobIndex,
			})

			if err != nil {
				log.Error("Unable to encode EigenDA message", "err", err)
				return nil, err
			}

			return msg, nil

		} else if statusRes.Status == disperser.BlobStatus_UNKNOWN ||
			statusRes.Status == disperser.BlobStatus_FAILED {
			log.Error("EigenDA blob dispersal failed in processing", "requestID", base64RequestID, "err", err)
			return nil, fmt.Errorf("EigenDA blob dispersal failed in processing with reply status %d", statusRes.Status)
		}

		time.Sleep(time.Duration(m.StatusQueryRetryIntervalSeconds) * time.Second)
	}
	return nil, fmt.Errorf("timed out getting EigenDA status for dispersed blob key: %s", base64RequestID)

}
