//go:build !harness

// These tests will not be run if the harness build tag is set.

package eth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"decred.org/dcrdex/dex"
	"decred.org/dcrdex/dex/calc"
	"decred.org/dcrdex/dex/encode"
	dexeth "decred.org/dcrdex/dex/networks/eth"
	"decred.org/dcrdex/server/asset"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const initLocktime = 1632112916

var (
	_       ethFetcher = (*testNode)(nil)
	tLogger            = dex.StdOutLogger("ETHTEST", dex.LevelTrace)
	tCtx    context.Context

	initiatorAddr          = common.HexToAddress("0x2b84C791b79Ee37De042AD2ffF1A253c3ce9bc27")
	participantAddr        = common.HexToAddress("345853e21b1d475582E71cC269124eD5e2dD3422")
	secretA                = mustArray32("557cf82e5e72e8ba23963a5e2832767a7e2a3e0a58ac00d319605f4b34b46de2")
	secretHashA            = mustArray32("09439d8fdc46a777590a5345704042c2774061d5322c6a94352c98a6f6a3630a")
	secretB                = mustArray32("87eac09638c0c38b4e735b79f053cb869167ee770640ac5df5c4ab030813122a")
	secretHashB            = mustArray32("ebdc4c31b88d0c8f4d644591a8e00e92b607f920ad8050deb7c7469767d9c561")
	initValue       uint64 = 2_500_000_000
	swapVectorA            = &dexeth.SwapVector{
		From:       initiatorAddr,
		To:         participantAddr,
		Value:      dexeth.GweiToWei(initValue),
		SecretHash: secretHashA,
		LockTime:   initLocktime,
	}
	locatorA    = swapVectorA.Locator()
	swapVectorB = &dexeth.SwapVector{
		From:       initiatorAddr,
		To:         participantAddr,
		Value:      dexeth.GweiToWei(initValue),
		SecretHash: secretHashB,
		LockTime:   initLocktime,
	}
	locatorB = swapVectorB.Locator()

	initCalldataV0 = mustParseHex("a8793f94000000000000000000000000000000000000" +
		"0000000000000000000000000020000000000000000000000000000000000000000000" +
		"0000000000000000000002000000000000000000000000000000000000000000000000" +
		"000000006148111409439d8fdc46a777590a5345704042c2774061d5322c6a94352c98" +
		"a6f6a3630a000000000000000000000000345853e21b1d475582e71cc269124ed5e2dd" +
		"342200000000000000000000000000000000000000000000000022b1c8c1227a000000" +
		"00000000000000000000000000000000000000000000000000000061481114ebdc4c31" +
		"b88d0c8f4d644591a8e00e92b607f920ad8050deb7c7469767d9c56100000000000000" +
		"0000000000345853e21b1d475582e71cc269124ed5e2dd342200000000000000000000" +
		"000000000000000000000000000022b1c8c1227a0000")
	/* initCallData parses to:
	[ETHSwapInitiation {
			RefundTimestamp: 1632112916
			SecretHash: 8b3e4acc53b664f9cf6fcac0adcd328e95d62ba1f4379650ae3e1460a0f9d1a1
			Value: 2.5e9 gwei
			Participant: 0x345853e21b1d475582e71cc269124ed5e2dd3422
		},
	ETHSwapInitiation {
			RefundTimestamp: 1632112916
			SecretHash: ebdc4c31b88d0c8f4d644591a8e00e92b607f920ad8050deb7c7469767d9c561
			Value: 2.5e9 gwei
			Participant: 0x345853e21b1d475582e71cc269124ed5e2dd3422
		}]
	*/
	initCalldataV1 = mustParseHex("52145bc0000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000040000000000000000000000000000000000000000000000000" +
		"000000000000000209439d8fdc46a777590a5345704042c2774061d5322c6a94352c98" +
		"a6f6a3630a00000000000000000000000000000000000000000000000022b1c8c1227a" +
		"00000000000000000000000000002b84c791b79ee37de042ad2fff1a253c3ce9bc2700" +
		"0000000000000000000000000000000000000000000000000000006148111400000000" +
		"0000000000000000345853e21b1d475582e71cc269124ed5e2dd3422ebdc4c31b88d0c" +
		"8f4d644591a8e00e92b607f920ad8050deb7c7469767d9c56100000000000000000000" +
		"000000000000000000000000000022b1c8c1227a00000000000000000000000000002b" +
		"84c791b79ee37de042ad2fff1a253c3ce9bc2700000000000000000000000000000000" +
		"00000000000000000000000061481114000000000000000000000000345853e21b1d47" +
		"5582e71cc269124ed5e2dd3422")

	redeemCalldataV0 = mustParseHex("f4fd17f90000000000000000000000000000000000" +
		"0000000000000000000000000000200000000000000000000000000000000000000000" +
		"000000000000000000000002557cf82e5e72e8ba23963a5e2832767a7e2a3e0a58ac00" +
		"d319605f4b34b46de209439d8fdc46a777590a5345704042c2774061d5322c6a94352c" +
		"98a6f6a3630a87eac09638c0c38b4e735b79f053cb869167ee770640ac5df5c4ab0308" +
		"13122aebdc4c31b88d0c8f4d644591a8e00e92b607f920ad8050deb7c7469767d9c561")

	redeemCalldataV1 = mustParseHex("f9f2e0f40000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000000000000000000000000000000000" +
		"00000000000000000040000000000000000000000000000000000000000000000000000000" +
		"000000000209439d8fdc46a777590a5345704042c2774061d5322c6a94352c98a6f6a3630a" +
		"00000000000000000000000000000000000000000000000022b1c8c1227a00000000000000" +
		"000000000000002b84c791b79ee37de042ad2fff1a253c3ce9bc2700000000000000000000" +
		"00000000000000000000000000000000000061481114000000000000000000000000345853" +
		"e21b1d475582e71cc269124ed5e2dd3422557cf82e5e72e8ba23963a5e2832767a7e2a3e0a" +
		"58ac00d319605f4b34b46de2ebdc4c31b88d0c8f4d644591a8e00e92b607f920ad8050deb7" +
		"c7469767d9c56100000000000000000000000000000000000000000000000022b1c8c1227a" +
		"00000000000000000000000000002b84c791b79ee37de042ad2fff1a253c3ce9bc27000000" +
		"00000000000000000000000000000000000000000000000000614811140000000000000000" +
		"00000000345853e21b1d475582e71cc269124ed5e2dd342287eac09638c0c38b4e735b79f0" +
		"53cb869167ee770640ac5df5c4ab030813122a")
	/*
		redeemCallData parses to:
		[ETHSwapRedemption {
			SecretHash: 99d971975c09331eb00f5e0dc1eaeca9bf4ee2d086d3fe1de489f920007d6546
			Secret: 2c0a304c9321402dc11cbb5898b9f2af3029ce1c76ec6702c4cd5bb965fd3e73
		}
		ETHSwapRedemption {
			SecretHash: ebdc4c31b88d0c8f4d644591a8e00e92b607f920ad8050deb7c7469767d9c561
			Secret: 87eac09638c0c38b4e735b79f053cb869167ee770640ac5df5c4ab030813122a
		}]
	*/
)

func mustParseHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func mustArray32(s string) (b [32]byte) {
	copy(b[:], mustParseHex(s))
	return
}

type testNode struct {
	connectErr       error
	bestHdr          *types.Header
	bestHdrErr       error
	hdrByHeight      *types.Header
	hdrByHeightErr   error
	blkNum           uint64
	blkNumErr        error
	syncProg         *ethereum.SyncProgress
	syncProgErr      error
	suggGasTipCap    *big.Int
	suggGasTipCapErr error
	swp              map[string]*dexeth.SwapState
	swpErr           error
	tx               *types.Transaction
	txIsMempool      bool
	txErr            error
	receipt          *types.Receipt
	acctBal          *big.Int
	acctBalErr       error
}

func (n *testNode) connect(ctx context.Context) error {
	return n.connectErr
}

func (n *testNode) shutdown() {}

func (n *testNode) loadToken(context.Context, uint32, *VersionedToken) error {
	return nil
}

func (n *testNode) bestHeader(ctx context.Context) (*types.Header, error) {
	return n.bestHdr, n.bestHdrErr
}

func (n *testNode) headerByHeight(ctx context.Context, height uint64) (*types.Header, error) {
	return n.hdrByHeight, n.hdrByHeightErr
}

func (n *testNode) blockNumber(ctx context.Context) (uint64, error) {
	return n.blkNum, n.blkNumErr
}

func (n *testNode) syncProgress(ctx context.Context) (*ethereum.SyncProgress, error) {
	return n.syncProg, n.syncProgErr
}

func (n *testNode) suggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return n.suggGasTipCap, n.suggGasTipCapErr
}

func (n *testNode) status(ctx context.Context, assetID uint32, token common.Address, locator []byte) (*dexeth.SwapStatus, error) {
	if s := n.swp[string(locator)]; s != nil {
		return &dexeth.SwapStatus{
			BlockHeight: s.BlockHeight,
			Secret:      s.Secret,
			Step:        s.State,
		}, n.swpErr
	}
	return nil, n.swpErr
}

func (n *testNode) vector(ctx context.Context, assetID uint32, locator []byte) (*dexeth.SwapVector, error) {
	var secretHash [32]byte
	switch len(locator) {
	case dexeth.LocatorV1Length:
		vec, _ := dexeth.ParseV1Locator(locator)
		secretHash = vec.SecretHash
	default:
		copy(secretHash[:], locator)
	}

	if s := n.swp[string(locator)]; s != nil {
		return &dexeth.SwapVector{
			From:       s.Initiator,
			To:         s.Participant,
			Value:      s.Value,
			SecretHash: secretHash,
			LockTime:   uint64(s.LockTime.Unix()),
		}, n.swpErr
	}
	return nil, n.swpErr
}

func (n *testNode) statusAndVector(ctx context.Context, assetID uint32, locator []byte) (*dexeth.SwapStatus, *dexeth.SwapVector, error) {
	status, _ := n.status(ctx, assetID, common.Address{}, locator)
	vec, _ := n.vector(ctx, assetID, locator)
	return status, vec, n.swpErr
}

func (n *testNode) transaction(ctx context.Context, txHash common.Hash) (tx *types.Transaction, isMempool bool, err error) {
	return n.tx, n.txIsMempool, n.txErr
}

func (n *testNode) transactionReceipt(ctx context.Context, txHash common.Hash) (tx *types.Receipt, err error) {
	return n.receipt, nil
}

func (n *testNode) accountBalance(ctx context.Context, assetID uint32, addr common.Address) (*big.Int, error) {
	return n.acctBal, n.acctBalErr
}

func tSwap(bn, locktime int64, value uint64, secret [32]byte, state dexeth.SwapStep, participantAddr *common.Address) *dexeth.SwapState {
	return &dexeth.SwapState{
		Secret:      secret,
		BlockHeight: uint64(bn),
		LockTime:    time.Unix(locktime, 0),
		Participant: *participantAddr,
		State:       state,
		Value:       dexeth.GweiToWei(value),
	}
}

func tNewBackend(assetID uint32) (*AssetBackend, *testNode) {
	node := &testNode{
		swp: make(map[string]*dexeth.SwapState),
	}
	return &AssetBackend{
		baseBackend: &baseBackend{
			net:        dex.Simnet,
			node:       node,
			baseLogger: tLogger,
		},
		log:        tLogger.SubLogger(strings.ToUpper(dex.BipIDSymbol(assetID))),
		assetID:    assetID,
		blockChans: make(map[chan *asset.BlockUpdate]struct{}),
		atomize:    dexeth.WeiToGwei,
		gases:      dexeth.VersionedGases[ProtocolVersion(BipID).ContractVersion()],
	}, node
}

func TestMain(m *testing.M) {
	tLogger = dex.StdOutLogger("TEST", dex.LevelTrace)
	var shutdown func()
	tCtx, shutdown = context.WithCancel(context.Background())
	doIt := func() int {
		defer shutdown()
		dexeth.Tokens[usdcID].NetTokens[dex.Simnet].SwapContracts[0].Address = common.BytesToAddress(encode.RandomBytes(20))
		dexeth.Tokens[usdcID].NetTokens[dex.Simnet].SwapContracts[1].Address = common.BytesToAddress(encode.RandomBytes(20))
		dexeth.ContractAddresses[0][dex.Simnet] = common.BytesToAddress(encode.RandomBytes(20))
		dexeth.ContractAddresses[1][dex.Simnet] = common.BytesToAddress(encode.RandomBytes(20))
		return m.Run()
	}
	os.Exit(doIt())
}

func TestDecodeCoinID(t *testing.T) {
	drv := &Driver{}
	txid := "0x1b86600b740d58ecc06eda8eba1c941c7ba3d285c78be89b56678da146ed53d1"
	txHashB := mustParseHex("1b86600b740d58ecc06eda8eba1c941c7ba3d285c78be89b56678da146ed53d1")

	type test struct {
		name    string
		input   []byte
		wantErr bool
		expRes  string
	}

	tests := []test{{
		name:   "ok",
		input:  txHashB,
		expRes: txid,
	}, {
		name:    "too short",
		input:   txHashB[:len(txHashB)/2],
		wantErr: true,
	}, {
		name:    "too long",
		input:   append(txHashB, txHashB...),
		wantErr: true,
	}}

	for _, tt := range tests {
		res, err := drv.DecodeCoinID(tt.input)
		if err != nil {
			if !tt.wantErr {
				t.Fatalf("%s: error: %v", tt.name, err)
			}
			continue
		}

		if tt.wantErr {
			t.Fatalf("%s: no error", tt.name)
		}
		if res != tt.expRes {
			t.Fatalf("%s: wrong result. wanted %s, got %s", tt.name, tt.expRes, res)
		}
	}
}

func TestRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	backend, err := unconnectedETH(BipID, defaultProtocolVersion.ContractVersion(), common.Address{}, common.Address{}, registeredTokens, tLogger, dex.Simnet)
	if err != nil {
		t.Fatalf("unconnectedETH error: %v", err)
	}
	backend.node = &testNode{
		blkNum: backend.bestHeight + 1,
	}
	ch := backend.BlockChannel(1)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-time.After(time.Second * 2):
		}
	}()
	backend.run(ctx)
	// Ok if ctx was canceled above. Linters complain about calling t.Fatal
	// in the goroutine above.
	select {
	case <-ctx.Done():
		return
	default:
		t.Fatal("test timeout")
	}
}

func TestFeeRate(t *testing.T) {
	maxInt := ^uint64(0)
	maxWei := new(big.Int).SetUint64(maxInt)
	gweiFactorBig := big.NewInt(dexeth.GweiFactor)
	maxWei.Mul(maxWei, gweiFactorBig)
	overMaxWei := new(big.Int).Set(maxWei)
	overMaxWei.Add(overMaxWei, gweiFactorBig)
	tests := []struct {
		name             string
		hdrBaseFee       *big.Int
		hdrErr           error
		suggGasTipCap    *big.Int
		suggGasTipCapErr error
		wantFee          uint64
		wantErr          bool
	}{{
		name:          "ok zero",
		hdrBaseFee:    new(big.Int),
		suggGasTipCap: new(big.Int),
		wantFee:       0,
	}, {
		name:          "ok rounded up",
		hdrBaseFee:    big.NewInt(dexeth.GweiFactor - 1),
		suggGasTipCap: new(big.Int),
		wantFee:       2,
	}, {
		name:          "ok 100, 2",
		hdrBaseFee:    big.NewInt(dexeth.GweiFactor * 100),
		suggGasTipCap: big.NewInt(dexeth.GweiFactor * 2),
		wantFee:       202,
	}, {
		name:          "over max int",
		hdrBaseFee:    overMaxWei,
		suggGasTipCap: big.NewInt(dexeth.GweiFactor * 2),
		wantErr:       true,
	}, {
		name:          "node header err",
		hdrBaseFee:    new(big.Int),
		hdrErr:        errors.New(""),
		suggGasTipCap: new(big.Int),
		wantErr:       true,
	}, {
		name:          "nil base fee error",
		hdrBaseFee:    nil,
		suggGasTipCap: new(big.Int),
		wantErr:       true,
	}, {
		name:             "node suggest gas tip cap err",
		hdrBaseFee:       new(big.Int),
		suggGasTipCapErr: errors.New(""),
		wantErr:          true,
	}}

	for _, test := range tests {
		eth, node := tNewBackend(BipID)
		node.bestHdr = &types.Header{
			BaseFee: test.hdrBaseFee,
		}
		node.bestHdrErr = test.hdrErr
		node.suggGasTipCap = test.suggGasTipCap
		node.suggGasTipCapErr = test.suggGasTipCapErr

		fee, err := eth.FeeRate(tCtx)
		if test.wantErr {
			if err == nil {
				t.Fatalf("expected error for test %q", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for test %q: %v", test.name, err)
		}
		if fee != test.wantFee {
			t.Fatalf("want fee %v got %v for test %q", test.wantFee, fee, test.name)
		}
	}
}

func TestSynced(t *testing.T) {
	tests := []struct {
		name                    string
		syncProg                *ethereum.SyncProgress
		subSecs                 uint64
		bestHdrErr, syncProgErr error
		wantErr, wantSynced     bool
	}{{
		name:       "ok synced",
		subSecs:    dexeth.MaxBlockInterval - 1,
		wantSynced: true,
	}, {
		name:    "ok header too old",
		subSecs: dexeth.MaxBlockInterval,
	}, {
		name:       "best header error",
		bestHdrErr: errors.New(""),
		wantErr:    true,
	}}

	for _, test := range tests {
		nowInSecs := uint64(time.Now().Unix())
		eth, node := tNewBackend(BipID)
		node.syncProg = test.syncProg
		node.syncProgErr = test.syncProgErr
		node.bestHdr = &types.Header{Time: nowInSecs - test.subSecs}
		node.bestHdrErr = test.bestHdrErr

		synced, err := eth.Synced()
		if test.wantErr {
			if err == nil {
				t.Fatalf("expected error for test %q", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for test %q: %v", test.name, err)
		}
		if synced != test.wantSynced {
			t.Fatalf("want synced %v got %v for test %q", test.wantSynced, synced, test.name)
		}
	}
}

// TestRequiredOrderFunds ensures that a fee calculation in the calc package
// will come up with the correct required funds.
func TestRequiredOrderFunds(t *testing.T) {
	initTxSize := dexeth.InitGas(1, 0)
	swapVal := uint64(1000000000) // gwei
	numSwaps := uint64(17)        // swaps
	feeRate := uint64(30)         // gwei / gas

	// We want the fee calculation to simply be the cost of the gas used
	// for each swap plus the initial value.
	want := swapVal + (numSwaps * initTxSize * feeRate)
	// Second argument called inputsSize same as another initSize.
	got := calc.RequiredOrderFunds(swapVal, 0, numSwaps, initTxSize, initTxSize, feeRate)
	if got != want {
		t.Fatalf("want %v got %v for fees", want, got)
	}
}

func tTx(gasFeeCap, gasTipCap, value uint64, to *common.Address, data []byte) *types.Transaction {
	return types.NewTx(&types.DynamicFeeTx{
		GasFeeCap: dexeth.GweiToWei(gasFeeCap),
		GasTipCap: dexeth.GweiToWei(gasTipCap),
		To:        to,
		Value:     dexeth.GweiToWei(value),
		Data:      data,
	})
}

func TestContract(t *testing.T) {
	receiverAddr, contractAddr := new(common.Address), new(common.Address)
	copy(receiverAddr[:], encode.RandomBytes(20))
	copy(contractAddr[:], encode.RandomBytes(20))
	var txHash [32]byte
	copy(txHash[:], encode.RandomBytes(32))
	const gasPrice = 30
	const gasTipCap = 2
	const swapVal = 25e8
	const txVal = 5e9
	secret0, secret1, secretHash0, secretHash1 := secretA, secretB, secretHashA, secretHashB
	locator0, locator1 := locatorA, locatorB
	swaps := map[string]*dexeth.SwapState{
		string(secretHash0[:]): tSwap(97, initLocktime, swapVal, secret0, dexeth.SSInitiated, &participantAddr),
		string(secretHash1[:]): tSwap(97, initLocktime, swapVal, secret1, dexeth.SSInitiated, &participantAddr),
		string(locator1):       tSwap(97, initLocktime, swapVal, secret1, dexeth.SSInitiated, &participantAddr),
		string(locator0):       tSwap(97, initLocktime, swapVal, secret0, dexeth.SSInitiated, &participantAddr),
		string(locator1):       tSwap(97, initLocktime, swapVal, secret1, dexeth.SSInitiated, &participantAddr),
	}
	tests := []struct {
		name     string
		ver      uint32
		coinID   []byte
		tx       *types.Transaction
		locators [][]byte
		swapErr  error
		wantErr  bool
	}{
		{
			name:     "ok v0",
			tx:       tTx(gasPrice, gasTipCap, txVal, contractAddr, initCalldataV0),
			locators: [][]byte{secretHash0[:], secretHash1[:]},
			coinID:   txHash[:],
		}, {
			name:     "ok v1",
			ver:      1,
			tx:       tTx(gasPrice, gasTipCap, txVal, contractAddr, initCalldataV1),
			locators: [][]byte{locator0, locator1},
			coinID:   txHash[:],
		}, {
			name:     "new coiner error, wrong tx type",
			ver:      1,
			tx:       tTx(gasPrice, gasTipCap, txVal, contractAddr, initCalldataV1),
			locators: [][]byte{locator0, locator1},
			coinID:   txHash[1:],
			wantErr:  true,
		}, {
			name:     "confirmations error, swap error",
			ver:      1,
			tx:       tTx(gasPrice, gasTipCap, txVal, contractAddr, initCalldataV1),
			locators: [][]byte{locator0, locator1},
			coinID:   txHash[:],
			swapErr:  errors.New(""),
			wantErr:  true,
		},
	}
	for _, test := range tests {
		eth, node := tNewBackend(BipID)
		node.tx = test.tx
		node.swp = swaps
		node.swpErr = test.swapErr
		eth.contractAddr = *contractAddr

		for _, locator := range test.locators {
			contractData := dexeth.EncodeContractData(test.ver, locator)
			contract, err := eth.Contract(test.coinID, contractData)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error for test %q", test.name)
				}
				continue
			}
			if err != nil {
				t.Fatalf("unexpected error for test %q: %v", test.name, err)
			}
			if contract.SwapAddress != participantAddr.String() ||
				contract.LockTime.Unix() != initLocktime {
				t.Fatalf("returns do not match expected for test %q", test.name)
			}
		}
	}
}

func TestValidateFeeRate(t *testing.T) {
	swapCoin := swapCoin{
		baseCoin: &baseCoin{
			backend:   &AssetBackend{log: tLogger},
			gasFeeCap: dexeth.GweiToWei(100),
			gasTipCap: dexeth.GweiToWei(2),
		},
	}

	contract := &asset.Contract{
		Coin: &swapCoin,
	}

	eth, _ := tNewBackend(BipID)

	if !eth.ValidateFeeRate(contract.Coin, 100) {
		t.Fatalf("expected valid fee rate, but was not valid")
	}

	if eth.ValidateFeeRate(contract.Coin, 101) {
		t.Fatalf("expected invalid fee rate, but was valid")
	}

	swapCoin.gasTipCap = dexeth.GweiToWei(dexeth.MinGasTipCap - 1)
	if eth.ValidateFeeRate(contract.Coin, 100) {
		t.Fatalf("expected invalid fee rate, but was valid")
	}
}

func TestValidateSecret(t *testing.T) {
	v := &dexeth.SwapVector{SecretHash: sha256.Sum256(secretA[:]), Value: new(big.Int)}
	badV := &dexeth.SwapVector{SecretHash: [32]byte{}, Value: new(big.Int)}

	tests := []struct {
		name         string
		contractData []byte
		want         bool
	}{
		{
			name:         "ok v0",
			contractData: dexeth.EncodeContractData(0, secretHashA[:]),
			want:         true,
		}, {
			name:         "ok v1",
			contractData: dexeth.EncodeContractData(1, v.Locator()),
			want:         true,
		}, {
			name:         "not the right hash",
			contractData: dexeth.EncodeContractData(1, badV.Locator()),
		}, {
			name: "bad contract data",
		},
	}
	for _, test := range tests {
		eth, _ := tNewBackend(BipID)
		got := eth.ValidateSecret(secretA[:], test.contractData)
		if test.want != got {
			t.Fatalf("expected %v but got %v for test %q", test.want, got, test.name)
		}
	}
}

func TestRedemption(t *testing.T) {
	receiverAddr, contractAddr := new(common.Address), new(common.Address)
	copy(receiverAddr[:], encode.RandomBytes(20))
	copy(contractAddr[:], encode.RandomBytes(20))
	secret, secretHash := secretB, secretHashB
	var txHash [32]byte
	copy(txHash[:], encode.RandomBytes(32))
	const gasPrice = 30
	const gasTipCap = 2

	tests := []struct {
		name    string
		ver     uint32
		coinID  []byte
		locator []byte
		swp     *dexeth.SwapState
		tx      *types.Transaction
		wantErr bool
	}{
		{
			name:    "ok v0",
			tx:      tTx(gasPrice, gasTipCap, 0, contractAddr, redeemCalldataV0),
			locator: secretHash[:],
			coinID:  txHash[:],
			swp:     tSwap(0, 0, 0, secret, dexeth.SSRedeemed, receiverAddr),
		}, {
			name:    "ok v1",
			ver:     1,
			tx:      tTx(gasPrice, gasTipCap, 0, contractAddr, redeemCalldataV1),
			locator: locatorA,
			coinID:  txHash[:],
			swp:     tSwap(0, 0, 0, secret, dexeth.SSRedeemed, receiverAddr),
		}, {
			name:    "new coiner error, wrong tx type",
			tx:      tTx(gasPrice, gasTipCap, 0, contractAddr, redeemCalldataV0),
			locator: secretHash[:],
			coinID:  txHash[1:],
			wantErr: true,
		}, {
			name:    "confirmations error, swap wrong state",
			tx:      tTx(gasPrice, gasTipCap, 0, contractAddr, redeemCalldataV0),
			locator: secretHash[:],
			swp:     tSwap(0, 0, 0, secret, dexeth.SSRefunded, receiverAddr),
			coinID:  txHash[:],
			wantErr: true,
		}, {
			name:    "validate redeem error",
			tx:      tTx(gasPrice, gasTipCap, 0, contractAddr, redeemCalldataV0),
			locator: secretHash[:31],
			coinID:  txHash[:],
			swp:     tSwap(0, 0, 0, secret, dexeth.SSRedeemed, receiverAddr),
			wantErr: true,
		},
	}

	for _, test := range tests {
		eth, node := tNewBackend(BipID)
		node.tx = test.tx
		node.receipt = &types.Receipt{
			BlockNumber: big.NewInt(5),
		}
		node.swp[string(test.locator)] = test.swp
		eth.contractAddr = *contractAddr

		contract := dexeth.EncodeContractData(test.ver, test.locator)
		_, err := eth.Redemption(test.coinID, nil, contract)
		if test.wantErr {
			if err == nil {
				t.Fatalf("expected error for test %q", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for test %q: %v", test.name, err)
		}
	}
}

func TestTxData(t *testing.T) {
	eth, node := tNewBackend(BipID)

	const gasPrice = 30
	const gasTipCap = 2
	const value = 5e9
	addr := randomAddress()
	data := encode.RandomBytes(5)
	tx := tTx(gasPrice, gasTipCap, value, addr, data)
	goodCoinID, _ := hex.DecodeString("09c3bed75b35c6cf0549b0636c9511161b18765c019ef371e2a9f01e4b4a1487")
	node.tx = tx

	// initial success
	txData, err := eth.TxData(goodCoinID)
	if err != nil {
		t.Fatalf("TxData error: %v", err)
	}
	checkB, _ := tx.MarshalBinary()
	if !bytes.Equal(txData, checkB) {
		t.Fatalf("tx data not transmitted")
	}

	// bad coin ID
	coinID := encode.RandomBytes(2)
	_, err = eth.TxData(coinID)
	if err == nil {
		t.Fatalf("no error for bad coin ID")
	}

	// Wrong type of coin ID
	_, err = eth.TxData(goodCoinID[2:])
	if err == nil {
		t.Fatalf("no error for wrong coin type")
	}

	// No transaction
	node.tx = nil
	_, err = eth.TxData(goodCoinID)
	if err == nil {
		t.Fatalf("no error for missing tx")
	}

	// Success again
	node.tx = tx
	_, err = eth.TxData(goodCoinID)
	if err != nil {
		t.Fatalf("TxData error: %v", err)
	}
}

func TestValidateContract(t *testing.T) {
	t.Run("eth", func(t *testing.T) { testValidateContract(t, BipID) })
	t.Run("token", func(t *testing.T) { testValidateContract(t, usdcID) })
}

func testValidateContract(t *testing.T, assetID uint32) {
	badLoc := append(locatorA, 8)
	tests := []struct {
		name    string
		ver     uint32
		locator []byte
		wantErr bool
	}{
		{
			name:    "ok v0",
			locator: secretHashA[:],
		}, {
			name:    "ok v1",
			ver:     1,
			locator: locatorA[:],
		}, {
			name:    "wrong size",
			ver:     1,
			locator: badLoc,
			wantErr: true,
		}, {
			name:    "wrong version",
			ver:     0,
			locator: locatorA[:], // should be secretHashA
			wantErr: true,
		},
	}

	type contractValidator interface {
		ValidateContract([]byte) error
	}

	for _, test := range tests {
		eth, _ := tNewBackend(assetID)
		var cv contractValidator
		if assetID == BipID {
			cv = &ETHBackend{eth}
		} else {
			cv = &TokenBackend{
				AssetBackend: eth,
				VersionedToken: &VersionedToken{
					Token:           dexeth.Tokens[usdcID],
					ContractVersion: test.ver,
				},
			}
		}

		contractData := dexeth.EncodeContractData(test.ver, test.locator)
		err := cv.ValidateContract(contractData)
		if test.wantErr {
			if err == nil {
				t.Fatalf("expected error for test %q", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for test %q: %v", test.name, err)
		}
	}
}

func TestAccountBalance(t *testing.T) {
	eth, node := tNewBackend(BipID)

	const gweiBal = 1e9
	bigBal := big.NewInt(gweiBal)
	node.acctBal = bigBal.Mul(bigBal, big.NewInt(dexeth.GweiFactor))

	// Initial success
	bal, err := eth.AccountBalance("")
	if err != nil {
		t.Fatalf("AccountBalance error: %v", err)
	}

	if bal != gweiBal {
		t.Fatalf("wrong balance. expected %f, got %d", gweiBal, bal)
	}

	// Only error path.
	node.acctBalErr = errors.New("test error")
	_, err = eth.AccountBalance("")
	if err == nil {
		t.Fatalf("no AccountBalance error when expected")
	}
	node.acctBalErr = nil

	// Success again
	_, err = eth.AccountBalance("")
	if err != nil {
		t.Fatalf("AccountBalance error: %v", err)
	}
}

func TestPoll(t *testing.T) {
	tests := []struct {
		name        string
		addBlock    bool
		blockNumErr error
	}{{
		name: "ok nothing to do",
	}, {
		name:     "ok new",
		addBlock: true,
	}, {
		name:        "blockNumber error",
		blockNumErr: errors.New(""),
	}}

	for _, test := range tests {
		be, node := tNewBackend(BipID)
		eth := &ETHBackend{be}
		node.blkNumErr = test.blockNumErr
		if test.addBlock {
			node.blkNum = be.bestHeight + 1
		} else {
			node.blkNum = be.bestHeight
		}
		ch := make(chan *asset.BlockUpdate, 1)
		eth.blockChans[ch] = struct{}{}
		bu := new(asset.BlockUpdate)
		wait := make(chan struct{})
		go func() {
			select {
			case bu = <-ch:
			case <-time.After(time.Second * 2):
			}
			close(wait)
		}()
		eth.poll(nil)
		<-wait
		if test.blockNumErr != nil {
			if bu.Err == nil {
				t.Fatalf("expected error for test %q", test.name)
			}
			continue
		}
		if bu.Err != nil {
			t.Fatalf("unexpected error for test %q: %v", test.name, bu.Err)
		}
	}
}

func TestValidateSignature(t *testing.T) {
	// "ok" values used are the same as tests in client/assets/eth.
	pkBytes := mustParseHex("04b911d1f39f7792e165767e35aa134083e2f70ac7de6945d7641a3015d09a54561b71112b8d60f63831f0e62c23c6921ec627820afedf8236155b9e9bd82b6523")
	msg := []byte("msg")
	sigBytes := mustParseHex("ffd26911d3fdaf11ac44801744f2df015a16539b6e688aff4cabc092b747466e7bc8036a03d1479a1570dd11bf042120301c34a65b237267720ef8a9e56f2eb1")
	max32Bytes := mustParseHex("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	addr := "0x2b84C791b79Ee37De042AD2ffF1A253c3ce9bc27"
	eth := new(AssetBackend)

	tests := []struct {
		name                   string
		wantErr                bool
		pkBytes, sigBytes, msg []byte
		addr                   string
	}{{
		name:     "ok",
		pkBytes:  pkBytes,
		msg:      msg,
		addr:     addr,
		sigBytes: sigBytes,
	}, {
		name:    "sig wrong size",
		pkBytes: pkBytes,
		msg:     msg,
		addr:    addr,
		wantErr: true,
	}, {
		name:     "pubkey doesn't match address",
		pkBytes:  pkBytes,
		msg:      msg,
		addr:     addr[:21] + "a",
		sigBytes: sigBytes,
		wantErr:  true,
	}, {
		name:     "bad pubkey",
		pkBytes:  pkBytes[1:],
		msg:      msg,
		sigBytes: sigBytes,
		addr:     addr,
		wantErr:  true,
	}, {
		name:     "r too big",
		pkBytes:  pkBytes,
		msg:      msg,
		sigBytes: append(append([]byte{}, max32Bytes...), sigBytes[32:]...),
		addr:     addr,
		wantErr:  true,
	}, {
		name:     "s too big",
		pkBytes:  pkBytes,
		msg:      msg,
		sigBytes: append(append(append([]byte{}, sigBytes[:32]...), max32Bytes...), byte(1)),
		addr:     addr,
		wantErr:  true,
	}, {
		name:     "cannot verify signature, bad msg",
		pkBytes:  pkBytes,
		sigBytes: sigBytes,
		addr:     addr,
		wantErr:  true,
	}}

	for _, test := range tests {
		err := eth.ValidateSignature(test.addr, test.pkBytes, test.msg, test.sigBytes)
		if test.wantErr {
			if err == nil {
				t.Fatalf("expected error for test %q", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for test %q: %v", test.name, err)
		}
	}
}

func TestIsRemoteURL(t *testing.T) {
	for _, tt := range []struct {
		url        string
		wantRemote bool
	}{
		{"http://localhost:1234", false},
		{"http://127.0.0.1", false},
		{"http://127.0.0.1:1234", false},
		{"https://127.0.0.1:1234", false},
		{"https://decred.org", true},
		{"https://241.45.173.171", true},
		{"http://[::1]:8080", false},
		{"https://[2001:db8::1]:8080", true},
		{"https://241.45.173.171:1234", true},
	} {
		uri, _ := url.Parse(tt.url)
		if is := isRemoteURL(uri); is != tt.wantRemote {
			t.Fatalf("%s: wanted %t, got %t", tt.url, tt.wantRemote, is)
		}
	}
}

func TestParseEndpoints(t *testing.T) {
	type test struct {
		name              string
		fileContents      string
		relayAddr         string
		expectedEndpoints []string
		wantErr           bool
	}

	url1 := "http://127.0.0.1:1234"
	url2 := "https://example.com"
	relayAddr := "123.111.4.8:1111"
	relayURL := "http://" + relayAddr

	tests := []*test{
		{
			name:              "single localhost in file",
			fileContents:      url1,
			expectedEndpoints: []string{"http://127.0.0.1:1234"},
		},
		{
			name:    "no path provided error",
			wantErr: true,
		},
		{
			name:              "two from file and a noderelay",
			fileContents:      url1 + "\n" + url2,
			relayAddr:         relayAddr,
			expectedEndpoints: []string{relayURL, url1, url2},
		},
		{
			name:              "just a relay adddress",
			relayAddr:         relayAddr,
			expectedEndpoints: []string{relayURL},
		},
	}

	runTest := func(t *testing.T, tt *test) {
		var configPath string
		if tt.fileContents != "" {
			f, err := os.CreateTemp("", "")
			if err != nil {
				t.Fatalf("error getting temporary file")
			}
			configPath = f.Name()
			defer os.Remove(configPath)
			defer f.Close()
			f.WriteString(tt.fileContents)
		}
		endpoints, err := parseEndpoints(&asset.BackendConfig{
			ConfigPath: configPath,
			RelayAddr:  tt.relayAddr,
		})
		if err != nil {
			if tt.wantErr {
				return
			}
			t.Fatalf("parseEndpoints error: %v", err)
		}
		if len(endpoints) != len(tt.expectedEndpoints) {
			t.Fatalf("wrong number of endpoints. wanted %d, got %d", len(tt.expectedEndpoints), len(endpoints))
		}
		for i, pt := range endpoints {
			if expURL := tt.expectedEndpoints[i]; pt.url != expURL {
				t.Fatalf("wrong endpoint at index %d: wanted %s, got %s", i, expURL, pt.url)
			}
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runTest(t, tt)
		})
	}
}
