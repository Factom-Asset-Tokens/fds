package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Factom-Asset-Tokens/factom"
)

// Generate a set of Data Store Chain Entries for the data read from cData.
//
// The data from cData may be compressed using zlib or gzip, and if so,
// compression must be initialized with the correct Format and Size. See
// Compression for more details.
//
// The given size must be the uncompressed size of the data.
//
// The given dataHash must be the sha256d hash of the uncompressed data.
//
// The appMetadata and appNamespace are optional. The appMetadata is included
// in the JSON stored in the content of the First Entry, and the appNamespace
// is appended to the ExtIDs of the First Entry and so must be known, along
// with the dataHash, for a client to recompute the ChainID.
//
// The returned Entry list may be submitted to Factom in any order so long as
// the first entry is submitted first as the chain creation entry.
func Generate(ctx context.Context, c *factom.Client, es factom.EsAddress,
	cData io.Reader, compression *Compression,
	size uint64, dataHash *factom.Bytes32,
	appMetadata json.RawMessage, appNamespace ...factom.Bytes) (
	factom.Bytes32, error) {

	m := Metadata{
		DataHash:    dataHash,
		Size:        size,
		Compression: compression,
		AppMetadata: appMetadata,
	}
	nameIDs := NameIDs(dataHash, appNamespace...)
	chainID := factom.ComputeChainID(nameIDs)

	if compression != nil {
		size = compression.Size
	}

	cDataBuf := bytes.NewBuffer(make([]byte, 0, size))

	n, err := cDataBuf.ReadFrom(cData)
	if err != nil {
		return factom.Bytes32{}, err
	}
	if n != int64(size) {
		return factom.Bytes32{}, fmt.Errorf("invalid size")
	}

	// Compute the expected DB Entry Count.
	dbECount := int(size / factom.EntryMaxDataLen)
	if size%factom.EntryMaxDataLen > 0 {
		dbECount++
	}

	// Compute the expected DBI Entry Count
	dbiECount := dbECount / MaxLinkedDBIEHashCount
	if dbECount%MaxLinkedDBIEHashCount > (MaxDBIEHashCount - MaxLinkedDBIEHashCount) {
		dbiECount++
	}

	entries := make(map[factom.Bytes32]factom.Bytes, dbiECount+dbECount)
	dbi := make([]byte, dbECount*32)

	e := factom.Entry{ChainID: &chainID}
	// Generate all Data Blocks and the DBI
	for i := 0; i < dbECount; i++ {
		e.Content = cDataBuf.Next(factom.EntryMaxDataLen)

		data, err := e.MarshalBinary()
		if err != nil {
			return factom.Bytes32{}, err
		}

		hash := factom.ComputeEntryHash(data)
		copy(dbi[i*32:], hash[:])
		entries[hash] = data
	}

	nDBHash := dbECount % MaxLinkedDBIEHashCount
	if nDBHash <= 2 {
		nDBHash += MaxLinkedDBIEHashCount
	}
	dbiI := len(dbi) - (nDBHash * 32)
	var hash factom.Bytes32
	m.DBIStart = &hash
	for i := 0; i < dbiECount; i++ {
		e.Content = dbi[dbiI:]

		dbi = dbi[:dbiI]
		dbiI -= MaxLinkedDBIEHashCount * 32

		data, err := e.MarshalBinary()
		if err != nil {
			return factom.Bytes32{}, err
		}

		hash = factom.ComputeEntryHash(data)
		entries[hash] = data

		e.ExtIDs = []factom.Bytes{hash[:]}
	}

	data, err := json.Marshal(m)
	if err != nil {
		return factom.Bytes32{}, err
	}

	firstE := factom.Entry{
		ChainID: &chainID,
		ExtIDs:  nameIDs,
		Content: data,
	}

	reveal, err := firstE.MarshalBinary()
	if err != nil {
		return factom.Bytes32{}, err
	}
	firstHash := factom.ComputeEntryHash(data)
	commit, _ := factom.GenerateCommit(es, reveal, &firstHash, true)
	if err := c.Commit(ctx, commit); err != nil {
		return factom.Bytes32{}, err
	}
	if err := c.Reveal(ctx, reveal); err != nil {
		return factom.Bytes32{}, err
	}

	for hash, reveal := range entries {
		commit, _ := factom.GenerateCommit(es, reveal, &hash, false)
		if err := c.Commit(ctx, commit); err != nil {
			return factom.Bytes32{}, err
		}
		if err := c.Reveal(ctx, reveal); err != nil {
			return factom.Bytes32{}, err
		}
	}

	return chainID, nil
}
