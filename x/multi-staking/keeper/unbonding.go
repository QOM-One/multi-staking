package keeper

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/realio-tech/multi-staking-module/x/multi-staking/types"
)

func (k Keeper) BurnToken(ctx sdk.Context, accAddr sdk.AccAddress, token sdk.Coins) error {
	err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, accAddr, types.ModuleName, token)
	if err != nil {
		return err
	}
	err = k.bankKeeper.BurnCoins(ctx, types.ModuleName, token)
	if err != nil {
		return err
	}
	return nil
}

func (k Keeper) GetUnlockEntryAtHeight(ctx sdk.Context, unlockID []byte, creationHeight int64) (types.UnlockEntry, bool) {
	// get unbonded record
	ubd, found := k.GetMultiStakingUnlock(ctx, unlockID)
	if !found {
		return types.UnlockEntry{}, false
	}
	var (
		unlockEntry      types.UnlockEntry
		foundUnlockEntry bool = false
	)

	for _, entry := range ubd.Entries {
		if entry.CreationHeight == creationHeight {
			unlockEntry = entry
			foundUnlockEntry = true
			break
		}
	}
	if !foundUnlockEntry {
		return types.UnlockEntry{}, false
	}

	return unlockEntry, foundUnlockEntry
}

func (k Keeper) CompleteUnbonding(
	ctx sdk.Context,
	intermediaryAcc sdk.AccAddress,
	valAddr sdk.ValAddress,
	unbondingHeight int64,
	balance math.Int,
) (unlockedAmount sdk.Coins, err error) {
	// get delAddr
	delAddr := types.DelegatorAccount(intermediaryAcc)

	// get unlock record
	unlockID := types.MultiStakingUnlockID(delAddr, valAddr)
	unlockEntry, found := k.GetUnlockEntryAtHeight(ctx, unlockID, unbondingHeight)
	if !found {
		return sdk.Coins{}, fmt.Errorf("unlock entry not found")
	}

	unlockDenom := k.GetValidatorAllowedToken(ctx, valAddr)
	unlockMultiStakingAmount := sdk.NewDecFromInt(balance).Mul(unlockEntry.ConversionRatio).RoundInt()

	// check amount
	if unlockMultiStakingAmount.GT(unlockEntry.Balance) {
		return unlockedAmount, fmt.Errorf("unlock amount greater than lock amount")
	}

	// burn bonded token
	burnToken := sdk.NewCoins(sdk.NewCoin(k.stakingKeeper.BondDenom(ctx), balance))
	k.BurnToken(ctx, intermediaryAcc, burnToken)

	// check unbond amount has been slashed or not
	if unbondEntry.Balance.GT(unlockMultiStakingAmount) {
		unlockedAmount = sdk.NewCoins(sdk.NewCoin(unlockDenom, unlockMultiStakingAmount))

		// Slash user amount
		burnUserAmount := sdk.NewCoins(sdk.NewCoin(unlockDenom, unbondEntry.Balance.Sub(unlockMultiStakingAmount)))
		err = k.bankKeeper.SendCoinsFromAccountToModule(ctx, intermediaryAcc, types.ModuleName, burnUserAmount)
		if err != nil {
			return unlockedAmount, err
		}
		err = k.bankKeeper.BurnCoins(ctx, types.ModuleName, burnUserAmount)
		if err != nil {
			return unlockedAmount, err
		}
	} else {
		unlockedAmount = sdk.NewCoins(sdk.NewCoin(unlockDenom, unbondEntry.Balance))
	}

	err = k.bankKeeper.SendCoins(ctx, intermediaryAcc, delAddr, unlockedAmount)
	if err != nil {
		return unlockedAmount, err
	}

	return unlockedAmount, nil
}
