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
// The size must be the size of the uncompressed data.
//
// The given dataHash must be the sha256d hash of the uncompressed data.
//
// The appMetadata and appNamespace are optional. The appMetadata is included
// in the JSON stored in the content of the First Entry, and the appNamespace
// is appended to the ExtIDs of the First Entry and so must be known, along
// with the dataHash, for a client to recompute the ChainID.
//
// The new Data Store chainID is returned, along with the commits and reveals
// required to create the Data Store Chain, and the totalCost in Entry Credits
// of creating the Data Store.
func Generate(ctx context.Context, c *factom.Client, es factom.EsAddress,
	cData io.Reader, compression *Compression,
	dataSize uint64, dataHash *factom.Bytes32,
	appMetadata json.RawMessage, appNamespace ...factom.Bytes) (

	chainID factom.Bytes32,
	commits, reveals []factom.Bytes,
	totalCost uint,
	err error) {

	// Compute Data Store ChainID.
	nameIDs := NameIDs(dataHash, appNamespace...)
	chainID = factom.ComputeChainID(nameIDs)

	// size of the data written to the chain.
	size := dataSize
	if compression != nil {
		size = compression.Size
	}

	// Read all cData into a Buffer.
	cDataBuf := bytes.NewBuffer(make([]byte, 0, size))
	n, err := cDataBuf.ReadFrom(cData)
	if err != nil {
		return factom.Bytes32{}, nil, nil, 0, err
	}
	if n != int64(size) {
		return factom.Bytes32{}, nil, nil, 0, fmt.Errorf("invalid size")
	}

	// Compute the expected Data Block Entry Count.
	dbECount := int(size / factom.EntryMaxDataLen)
	if size%factom.EntryMaxDataLen > 0 {
		dbECount++
	}

	// Compute the expected Data Block Index Entry Count
	dbiECount := dbECount / MaxLinkedDBIEHashCount
	if dbECount%MaxLinkedDBIEHashCount > (MaxDBIEHashCount - MaxLinkedDBIEHashCount) {
		dbiECount++
	}
	totalECount := 1 + dbiECount + dbECount

	// We return the commit and reveal data so that users of the library
	// don't need to regenerate them.
	commits = make([]factom.Bytes, totalECount)
	reveals = make([]factom.Bytes, totalECount)

	// The raw DBI, the concatenation of all Data Block Entry Hashes.
	dbi := make([]byte, dbECount*32)

	// Generate all Data Blocks and the DBI
	for i := 0; i < dbECount; i++ {
		e := factom.Entry{ChainID: &chainID}
		e.Content = cDataBuf.Next(factom.EntryMaxDataLen)

		reveal, err := e.MarshalBinary()
		if err != nil {
			return factom.Bytes32{}, nil, nil, 0, err
		}

		cost, _ := factom.EntryCost(len(reveal), false)
		totalCost += uint(cost)

		hash := factom.ComputeEntryHash(reveal)

		commit, _ := factom.GenerateCommit(es, reveal, &hash, false)

		copy(dbi[i*32:], hash[:])

		reveals[1+dbiECount+i] = reveal
		commits[1+dbiECount+i] = commit
	}

	// nDBHash is the number of trailing Data Block Entry Hashes from the
	// end of the DBI to include in the last entry. We populate the DBI
	// Entries in reverse order for creation of the linked list.
	nDBHash := dbECount % MaxLinkedDBIEHashCount
	if nDBHash <= 2 {
		nDBHash += MaxLinkedDBIEHashCount
	}

	// dbiI is the starting byte index of the dbi that we will include in
	// the last DBI Entry.
	dbiI := len(dbi) - (nDBHash * 32)

	var dbiStart factom.Bytes32
	for i := dbiECount; i > 0; i-- {
		e := factom.Entry{ChainID: &chainID}

		if !dbiStart.IsZero() {
			e.ExtIDs = []factom.Bytes{dbiStart[:]}
		}

		e.Content = dbi[dbiI:]
		dbi = dbi[:dbiI]

		dbiI -= MaxLinkedDBIEHashCount * 32

		reveal, err := e.MarshalBinary()
		if err != nil {
			return factom.Bytes32{}, nil, nil, 0, err
		}

		cost, _ := factom.EntryCost(len(reveal), false)
		totalCost += uint(cost)

		dbiStart = factom.ComputeEntryHash(reveal)

		commit, _ := factom.GenerateCommit(es, reveal, &dbiStart, false)

		reveals[i] = reveal
		commits[i] = commit
	}

	// Initialize Metadata for what will be the first entry.
	m := Metadata{
		Version:     Version,
		DataHash:    dataHash,
		Size:        dataSize,
		Compression: compression,
		AppMetadata: appMetadata,
		DBIStart:    &dbiStart,
	}

	firstE := factom.Entry{
		ChainID: &chainID,
		ExtIDs:  nameIDs,
	}
	firstE.Content, err = json.Marshal(m)
	if err != nil {
		return factom.Bytes32{}, nil, nil, 0, err
	}

	reveal, err := firstE.MarshalBinary()
	if err != nil {
		return factom.Bytes32{}, nil, nil, 0, err
	}
	hash := factom.ComputeEntryHash(reveal)
	commit, _ := factom.GenerateCommit(es, reveal, &hash, true)

	cost, _ := factom.EntryCost(len(reveal), true)
	totalCost += uint(cost)

	commits[0] = commit
	reveals[0] = reveal

	return chainID, commits, reveals, totalCost, nil
}
