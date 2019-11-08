package datastore

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Factom-Asset-Tokens/factom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	RunTestCreate bool
	c             = factom.NewClient()
	ecEs          = func() ECEsAddress {
		var ecEs ECEsAddress
		ecEs.Set("EC3emTZegtoGuPz3MRA4uC8SU6Up52abqQUEKqW44TppGGAc4Vrq")
		return ecEs
	}()
)

type ECEsAddress struct {
	EC factom.ECAddress
	Es factom.EsAddress
}

func (e *ECEsAddress) Set(adrStr string) error {
	if err := e.EC.Set(adrStr); err != nil {
		if err := e.Es.Set(adrStr); err != nil {
			return err
		}
		e.EC = e.Es.ECAddress()
	}
	return nil
}

func (e ECEsAddress) String() string {
	return e.EC.String()
}

func init() {
	rand.Seed(time.Now().Unix())
	flag.BoolVar(&RunTestCreate, "create", false, "Run the Create test")
	flag.StringVar(&c.FactomdServer, "factomd", c.FactomdServer,
		"factomd API endpoint")
	flag.Var(&ecEs, "ecadr", "Es or EC address to query from factom-walletd")
	flag.StringVar(&c.WalletdServer, "walletd", c.WalletdServer,
		"factom-walletd API endpoint")
	c.Factomd.Timeout = 10 * time.Second
	c.Walletd.Timeout = 10 * time.Second
}

func TestMain(m *testing.M) {
	flag.Parse()
	m.Run()
}

func TestGenerate(t *testing.T) {
	require := require.New(t)

	// Get Es Address
	if factom.Bytes32(ecEs.Es).IsZero() {
		es, err := ecEs.EC.GetEsAddress(nil, c)
		require.NoError(err)
		ecEs.Es = es
	}

	size := factom.EntryMaxDataLen * 10
	data := make([]byte, size)
	rand.Read(data)

	dataHash := factom.Bytes32(sha256.Sum256(data))
	dataHash = sha256.Sum256(dataHash[:])

	dataBuf := bytes.NewBuffer(data)
	cDataBuf := bytes.NewBuffer(make([]byte, 0, len(data)))

	gz := gzip.NewWriter(cDataBuf)
	_, err := dataBuf.WriteTo(gz)
	require.NoError(err)
	err = gz.Close()
	require.NoError(err)

	compression := Compression{Format: "gzip", Size: uint64(cDataBuf.Len())}

	fmt.Println("Generating data store:")
	fmt.Println("\tdataHash:", dataHash)
	fmt.Println("\tsize:", size)
	fmt.Println("\tcompression:", compression)
	chainID, commits, reveals, totalCost, err := Generate(nil, c, ecEs.Es,
		cDataBuf, &compression, uint64(size), &dataHash, nil)
	require.NoError(err)
	fmt.Println("\tChainID:", chainID)
	fmt.Println("\tEntry Count:", len(reveals))
	fmt.Println("\tTotal Cost:", totalCost)
	assert := assert.New(t)
	assert.Len(commits, 13)
	assert.Len(reveals, 13)
	assert.EqualValues(113, int(totalCost))
	assert.False(chainID.IsZero())

	if !RunTestCreate {
		t.SkipNow()
	}

	fmt.Println("Creating data store...")

	ecBal, err := ecEs.EC.GetBalance(nil, c)
	require.NoError(err)

	require.LessOrEqual(uint64(totalCost), ecBal, "insufficient balance")

	for _, commit := range commits {
		require.NoError(c.Commit(nil, commit))
	}
	for _, reveal := range reveals {
		require.NoError(c.Reveal(nil, reveal))
	}

	newECBal, err := ecEs.EC.GetBalance(nil, c)
	require.NoError(err)
	require.Equal(ecBal-uint64(totalCost), newECBal)
}
