package simapp

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/log"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	servercmtlog "github.com/cosmos/cosmos-sdk/server/log"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/testutil/testnet"
	sdk "github.com/cosmos/cosmos-sdk/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/stretchr/testify/require"
)

func TestTestnet(t *testing.T) {
	const nVals = 2
	const chainID = "simapp-chain"

	valPKs := testnet.NewValidatorPrivKeys(nVals)
	cmtVals := valPKs.CometGenesisValidators()
	stakingVals, _ := cmtVals.StakingValidators()

	b := testnet.NewGenesisBuilder().
		ChainID(chainID).
		DefaultAuthParams().
		Consensus(nil, cmtVals).
		BaseAccounts(stakingVals.BaseAccounts(), nil).
		StakingWithDefaultParams(nil, nil).
		BankingWithDefaultParams(stakingVals.Balances(), nil, nil, nil).
		DefaultDistribution().
		DefaultMint().
		SlashingWithDefaultParams(nil, nil)

	for i, v := range valPKs {
		b.GenTx(*v.Del, cmtVals[i].V, sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction))
	}

	jGenesis := b.Encode()

	logger := log.NewTestLogger(t)

	nodes := make([]*node.Node, nVals)
	p2pAddrs := make([]string, 0, nVals)
	for i := 0; i < nVals; i++ {
		dir := t.TempDir()

		// Obviously hardcoding the ports here is not good.
		// This test is passing currently, and it's a helpful reference
		// for the dynamic port assignment used in the CometStarter type.
		// Once that type is all put together, this test will be deleted.
		p2pPort := 30000 + i

		cmtCfg := cmtcfg.DefaultConfig()
		cmtCfg.RPC.ListenAddress = ""                                         // Do not run RPC service.
		cmtCfg.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", p2pPort) // Listen on random port for P2P too.
		cmtCfg.P2P.PersistentPeers = strings.Join(p2pAddrs, ",")
		cmtCfg.P2P.AllowDuplicateIP = true // All peers will be on 127.0.0.1.
		cmtCfg.P2P.AddrBookStrict = false

		cmtCfg.BaseConfig.DBBackend = "memdb"

		cfg, err := testnet.NewDiskConfig(dir, cmtCfg)
		require.NoError(t, err)

		appGenesisProvider := func() (*cmttypes.GenesisDoc, error) {
			appGenesis, err := genutiltypes.AppGenesisFromFile(cfg.Cfg.GenesisFile())
			if err != nil {
				return nil, err
			}

			return appGenesis.ToGenesisDoc()
		}

		err = os.WriteFile(cfg.Cfg.GenesisFile(), jGenesis, 0600)
		require.NoError(t, err)

		app := NewSimApp(
			logger.With("instance", i),
			dbm.NewMemDB(),
			nil,
			true,
			simtestutil.AppOptionsMap{},
			baseapp.SetChainID(chainID),
		)

		fpv := privval.NewFilePV(valPKs[i].Val, cfg.Cfg.PrivValidatorKeyFile(), cfg.Cfg.PrivValidatorStateFile())
		fpv.Save()

		n, err := node.NewNode(
			cfg.Cfg,
			fpv,
			cfg.NodeKey,
			proxy.NewLocalClientCreator(app),
			appGenesisProvider,
			node.DefaultDBProvider,
			node.DefaultMetricsProvider(cfg.Cfg.Instrumentation),
			servercmtlog.CometZeroLogWrapper{Logger: logger.With("rootmodule", fmt.Sprintf("comet_node-%d", i))},
		)
		if err != nil {
			t.Fatal(err)
		}

		require.NoError(t, n.Start())
		defer n.Stop()

		nodes[i] = n

		p2pAddr := fmt.Sprintf("%s@127.0.0.1:%d", n.Switch().NetAddress().ID, p2pPort)
		p2pAddrs = append(p2pAddrs, p2pAddr)
	}

	heightAdvanced := false
	for i := 0; i < 20; i++ {
		h := nodes[0].ConsensusState().GetLastHeight()
		if h < 2 {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		t.Logf("Saw height advance to %d", h)

		// Saw height advance.
		heightAdvanced = true
		break
	}

	if !heightAdvanced {
		t.Fatalf("consensus height did not advance in approximately 10 seconds")
	}
}
