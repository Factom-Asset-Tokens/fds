package datastore

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/Factom-Asset-Tokens/factom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	chainID := factom.NewBytes32(
		"06232c3c940b3a92abb7bf046b4ccf9b8c00725a157e95057bedef7e8b78ad2c")
	dataHash := factom.NewBytes32(
		"45193e8f2b9809e5dcfa57ff32cd4ea3c8ad1d062d9d7f2f8fc7fa63b73a02a0")
	m, err := Lookup(nil, c, &chainID)
	require.NoError(err)

	assert.Equal(dataHash, *m.DataHash)
	assert.EqualValues(102400, m.Size)
	if assert.NotNil(m.Compression) {
		assert.EqualValues(102458, m.Compression.Size)
	}

	dataBuf := bytes.NewBuffer(make([]byte, 0, m.Size))
	require.NoError(m.Download(nil, c, dataBuf))

	hash := sha256.Sum256(dataBuf.Bytes())
	hash = sha256.Sum256(hash[:])

	assert.EqualValues(dataHash, hash)
}
