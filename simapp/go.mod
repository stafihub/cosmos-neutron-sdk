module cosmossdk.io/simapp

go 1.19

require (
	cosmossdk.io/api v0.3.1
	cosmossdk.io/core v0.5.1
	cosmossdk.io/depinject v1.0.0-alpha.4
	cosmossdk.io/math v1.2.0
	cosmossdk.io/tools/rosetta v0.2.1
	github.com/cometbft/cometbft v0.37.2
	github.com/cometbft/cometbft-db v0.8.0
	github.com/cosmos/cosmos-sdk v0.47.5
	github.com/golang/mock v1.6.0
	github.com/spf13/cast v1.5.1
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.15.0
	github.com/stretchr/testify v1.8.4
	google.golang.org/protobuf v1.31.0
)

replace (
	// use cosmos fork of keyring
	github.com/99designs/keyring => github.com/cosmos/keyring v1.2.0
	// Simapp always use the latest version of the cosmos-sdk
	github.com/cosmos/cosmos-sdk => ../.
	// Fix upstream GHSA-h395-qcrw-5vmq and GHSA-3vp4-m3rf-835h vulnerabilities.
	// TODO Remove it: https://github.com/cosmos/cosmos-sdk/issues/10409
	github.com/gin-gonic/gin => github.com/gin-gonic/gin v1.9.0
	// replace broken goleveldb
	github.com/syndtr/goleveldb => github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
)
