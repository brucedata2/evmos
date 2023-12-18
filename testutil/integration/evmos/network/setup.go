// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)
package network

import (
	"fmt"
	"time"

	"github.com/evmos/evmos/v16/app"
	"github.com/evmos/evmos/v16/encoding"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/testutil/mock"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	simutils "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/evmos/evmos/v16/types"
	epochstypes "github.com/evmos/evmos/v16/x/epochs/types"
	evmtypes "github.com/evmos/evmos/v16/x/evm/types"
	infltypes "github.com/evmos/evmos/v16/x/inflation/v1/types"
)

// createValidatorSetAndSigners creates validator set with the amount of validators specified
// with the default power of 1.
func createValidatorSetAndSigners(numberOfValidators int) (*cmttypes.ValidatorSet, map[string]cmttypes.PrivValidator) {
	// Create validator set
	tmValidators := make([]*cmttypes.Validator, 0, numberOfValidators)
	signers := make(map[string]cmttypes.PrivValidator, numberOfValidators)

	for i := 0; i < numberOfValidators; i++ {
		privVal := mock.NewPV()
		pubKey, _ := privVal.GetPubKey()
		validator := cmttypes.NewValidator(pubKey, 1)
		tmValidators = append(tmValidators, validator)
		signers[pubKey.Address().String()] = privVal
	}

	return cmttypes.NewValidatorSet(tmValidators), signers
}

// createGenesisAccounts returns a slice of genesis accounts from the given
// account addresses.
func createGenesisAccounts(accounts []sdktypes.AccAddress) []authtypes.GenesisAccount {
	numberOfAccounts := len(accounts)
	genAccounts := make([]authtypes.GenesisAccount, 0, numberOfAccounts)
	for _, acc := range accounts {
		baseAcc := authtypes.NewBaseAccount(acc, nil, 0, 0)
		genAccounts = append(genAccounts, baseAcc)
	}
	return genAccounts
}

// createBalances creates balances for the given accounts and coin
func createBalances(accounts []sdktypes.AccAddress, coin sdktypes.Coin) []banktypes.Balance {
	numberOfAccounts := len(accounts)
	fundedAccountBalances := make([]banktypes.Balance, 0, numberOfAccounts)
	for _, acc := range accounts {
		balance := banktypes.Balance{
			Address: acc.String(),
			Coins:   sdktypes.NewCoins(coin),
		}

		fundedAccountBalances = append(fundedAccountBalances, balance)
	}
	return fundedAccountBalances
}

// createEvmosApp creates an evmos app
func createEvmosApp(chainID string) *app.Evmos {
	// Create evmos app
	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := true
	skipUpgradeHeights := map[int64]bool{}
	homePath := app.DefaultNodeHome
	invCheckPeriod := uint(5)
	encodingConfig := encoding.MakeConfig(app.ModuleBasics)
	appOptions := simutils.NewAppOptionsWithFlagHome(app.DefaultNodeHome)
	baseAppOptions := []func(*baseapp.BaseApp){baseapp.SetChainID(chainID)}

	return app.NewEvmos(
		logger,
		db,
		nil,
		loadLatest,
		skipUpgradeHeights,
		homePath,
		invCheckPeriod,
		encodingConfig,
		appOptions,
		baseAppOptions...,
	)
}

// createStakingValidator creates a staking validator from the given tm validator and bonded
func createStakingValidator(val *cmttypes.Validator, bondedAmt sdkmath.Int) (stakingtypes.Validator, error) {
	pk, err := cryptocodec.FromTmPubKeyInterface(val.PubKey)
	if err != nil {
		return stakingtypes.Validator{}, err
	}

	pkAny, err := codectypes.NewAnyWithValue(pk)
	if err != nil {
		return stakingtypes.Validator{}, err
	}

	commission := stakingtypes.NewCommission(sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec())
	validator := stakingtypes.Validator{
		OperatorAddress:   sdktypes.ValAddress(val.Address).String(),
		ConsensusPubkey:   pkAny,
		Jailed:            false,
		Status:            stakingtypes.Bonded,
		Tokens:            bondedAmt,
		DelegatorShares:   sdkmath.LegacyOneDec(),
		Description:       stakingtypes.Description{},
		UnbondingHeight:   int64(0),
		UnbondingTime:     time.Unix(0, 0).UTC(),
		Commission:        commission,
		MinSelfDelegation: sdkmath.ZeroInt(),
	}
	return validator, nil
}

// createStakingValidators creates staking validators from the given tm validators and bonded
// amounts
func createStakingValidators(tmValidators []*cmttypes.Validator, bondedAmt sdkmath.Int) ([]stakingtypes.Validator, error) {
	amountOfValidators := len(tmValidators)
	stakingValidators := make([]stakingtypes.Validator, 0, amountOfValidators)
	for _, val := range tmValidators {
		validator, err := createStakingValidator(val, bondedAmt)
		if err != nil {
			return nil, err
		}
		stakingValidators = append(stakingValidators, validator)
	}
	return stakingValidators, nil
}

// createDelegations creates delegations for the given validators and account
func createDelegations(tmValidators []*cmttypes.Validator, fromAccount sdktypes.AccAddress) []stakingtypes.Delegation {
	amountOfValidators := len(tmValidators)
	delegations := make([]stakingtypes.Delegation, 0, amountOfValidators)
	for _, val := range tmValidators {
		delegation := stakingtypes.NewDelegation(fromAccount.String(), sdktypes.ValAddress(val.Address).String(), sdkmath.LegacyOneDec())
		delegations = append(delegations, delegation)
	}
	return delegations
}

// StakingCustomGenesisState defines the staking genesis state
type StakingCustomGenesisState struct {
	denom string

	validators  []stakingtypes.Validator
	delegations []stakingtypes.Delegation
}

// setStakingGenesisState sets the staking genesis state
func setStakingGenesisState(evmosApp *app.Evmos, genesisState types.GenesisState, overwriteParams StakingCustomGenesisState) types.GenesisState {
	// Set staking params
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = overwriteParams.denom

	stakingGenesis := stakingtypes.NewGenesisState(stakingParams, overwriteParams.validators, overwriteParams.delegations)
	genesisState[stakingtypes.ModuleName] = evmosApp.AppCodec().MustMarshalJSON(stakingGenesis)
	return genesisState
}

// setAuthGenesisState sets the auth genesis state
func setAuthGenesisState(evmosApp *app.Evmos, genesisState types.GenesisState, genAccounts []authtypes.GenesisAccount) types.GenesisState {
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccounts)
	genesisState[authtypes.ModuleName] = evmosApp.AppCodec().MustMarshalJSON(authGenesis)
	return genesisState
}

// setInflationGenesisState sets the inflation genesis state
func setInflationGenesisState(evmosApp *app.Evmos, genesisState types.GenesisState, customGenesis CustomGenesisState) (types.GenesisState, error) {
	var (
		inflGenesis *infltypes.GenesisState
		ok          bool
	)
	custGen, found := customGenesis[infltypes.ModuleName]
	if !found {
		inflationParams := infltypes.DefaultParams()
		inflationParams.EnableInflation = false
		defaultGen := infltypes.NewGenesisState(inflationParams, uint64(0), epochstypes.DayEpochID, 365, 0)
		inflGenesis = &defaultGen
	} else {
		if inflGenesis, ok = custGen.(*infltypes.GenesisState); !ok {
			return nil, fmt.Errorf("invalid type %T for inflation genesis state", custGen)
		}
	}

	genesisState[infltypes.ModuleName] = evmosApp.AppCodec().MustMarshalJSON(inflGenesis)
	return genesisState, nil
}

type BankCustomGenesisState struct {
	totalSupply sdktypes.Coins
	balances    []banktypes.Balance
}

// setBankGenesisState sets the bank genesis state
func setBankGenesisState(evmosApp *app.Evmos, genesisState types.GenesisState, overwriteParams BankCustomGenesisState) types.GenesisState {
	bankGenesis := banktypes.NewGenesisState(
		banktypes.DefaultGenesisState().Params,
		overwriteParams.balances,
		overwriteParams.totalSupply,
		[]banktypes.Metadata{},
		[]banktypes.SendEnabled{},
	)
	genesisState[banktypes.ModuleName] = evmosApp.AppCodec().MustMarshalJSON(bankGenesis)
	return genesisState
}

// setBankGenesisState sets the bank genesis state
func setEVMGenesisState(evmosApp *app.Evmos, genesisState types.GenesisState, customGenesis CustomGenesisState) (types.GenesisState, error) {
	custGen, found := customGenesis[evmtypes.ModuleName]
	if !found {
		return genesisState, nil
	}
	evmGenesis, ok := custGen.(*evmtypes.GenesisState)
	if !ok {
		return nil, fmt.Errorf("invalid type %T for evm genesis state", custGen)
	}
	genesisState[evmtypes.ModuleName] = evmosApp.AppCodec().MustMarshalJSON(evmGenesis)
	return genesisState, nil
}

// calculateTotalSupply calculates the total supply from the given balances
func calculateTotalSupply(fundedAccountsBalances []banktypes.Balance) sdktypes.Coins {
	totalSupply := sdktypes.NewCoins()
	for _, balance := range fundedAccountsBalances {
		totalSupply = totalSupply.Add(balance.Coins...)
	}
	return totalSupply
}

// addBondedModuleAccountToFundedBalances adds bonded amount to bonded pool module account and include it on funded accounts
func addBondedModuleAccountToFundedBalances(fundedAccountsBalances []banktypes.Balance, totalBonded sdktypes.Coin) []banktypes.Balance {
	return append(fundedAccountsBalances, banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   sdktypes.Coins{totalBonded},
	})
}
