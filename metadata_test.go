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
		"1c9e1fd5603bd4cf54f8e186ebb8c9d68ae68ded0484fa6543e5915b72ce8990")
	dataHash := factom.NewBytes32(
		"1e3fcf78089dd840d132e4f30c1f56072e35cd33e06de879de6e8e859bb00d29")
	m, err := Lookup(nil, c, &chainID)
	require.NoError(err)

	assert.Equal(dataHash, *m.DataHash)
	assert.EqualValues(10250240, int(m.Size))
	if assert.NotNil(m.Compression) {
		assert.EqualValues(10253393, int(m.Compression.Size))
	}

	dataBuf := bytes.NewBuffer(make([]byte, 0, m.Size))
	require.NoError(m.Download(nil, c, dataBuf))

	hash := sha256.Sum256(dataBuf.Bytes())
	hash = sha256.Sum256(hash[:])

	assert.EqualValues(dataHash, hash)
}
