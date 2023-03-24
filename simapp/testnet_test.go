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
		DefaultDistribution()

	for i, v := range valPKs {
		b.GenTx(*v.Del, cmtVals[i].V, sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction))
	}

	jGenesis := b.Encode()
	t.Logf("jGenesis: %s", jGenesis)

	logger := log.NewTestLogger(t)

	p2pAddrs := make([]string, 0, nVals)
	for i := 0; i < nVals; i++ {
		dir := t.TempDir()

		cmtCfg := cmtcfg.DefaultConfig()
		cmtCfg.RPC.ListenAddress = "tcp://127.0.0.1:0" // Listen on random port for RPC.
		cmtCfg.P2P.ListenAddress = "tcp://127.0.0.1:0" // Listen on random port for P2P too.
		cmtCfg.P2P.PersistentPeers = strings.Join(p2pAddrs, ",")
		cmtCfg.P2P.AllowDuplicateIP = true // All peers will be on 127.0.0.1.
		cmtCfg.P2P.AddrBookStrict = false

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
	}

	time.Sleep(5 * time.Second)
}
