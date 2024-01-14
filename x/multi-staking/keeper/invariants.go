package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/realio-tech/multi-staking-module/x/multi-staking/types"
)

// RegisterInvariants registers all staking invariants
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "module-accounts",
		ModuleAccountInvariants(k))
	ir.RegisterRoute(types.ModuleName, "validator-lock-denom",
		ValidatorLockDenomInvariants(k))
}

func ModuleAccountInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		totalLockCoinAmount := sdk.NewCoins()

		// calculate lock amount
		lockCoinAmount := sdk.NewCoins()
		k.MultiStakingLockIterator(ctx, func(stakingLock types.MultiStakingLock) bool {
			lockCoinAmount = lockCoinAmount.Add(sdk.NewCoin(stakingLock.LockedCoin.Denom, stakingLock.LockedCoin.Amount))
			return false
		})
		totalLockCoinAmount = totalLockCoinAmount.Add(lockCoinAmount...)

		// calculate unlocking amount
		unlockingCoinAmount := sdk.NewCoins()
		k.MultiStakingUnlockIterator(ctx, func(unlock types.MultiStakingUnlock) bool {
			for _, entry := range unlock.Entries {
				unlockingCoinAmount = unlockingCoinAmount.Add(sdk.NewCoin(entry.UnlockingCoin.Denom, entry.UnlockingCoin.Amount))
			}
			return false
		})
		totalLockCoinAmount = totalLockCoinAmount.Add(unlockingCoinAmount...)

		moduleAccount := authtypes.NewModuleAddress(types.ModuleName)
		escrowBalances := k.bankKeeper.GetAllBalances(ctx, moduleAccount)

		broken := !escrowBalances.IsEqual(totalLockCoinAmount)

		return sdk.FormatInvariant(
			types.ModuleName,
			"ModuleAccountInvariants",
			fmt.Sprintf(
				"\tescrow coins balances: %v\n"+
					"\ttotal lock coin amount: %v\n",
				escrowBalances, totalLockCoinAmount),
		), broken
	}
}

func ValidatorLockDenomInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			msg    string
			broken bool
		)

		var multiStakingLocks []types.MultiStakingLock
		k.MultiStakingLockIterator(ctx, func(stakingLock types.MultiStakingLock) bool {
			multiStakingLocks = append(multiStakingLocks, stakingLock)
			return false
		})

		for _, lock := range multiStakingLocks {
			valAddr := lock.LockID.ValAddr
			if valAllowedDenom := k.GetValidatorMultiStakingCoin(ctx, sdk.ValAddress(valAddr)); valAllowedDenom != lock.LockedCoin.Denom {
				broken = true
				msg += fmt.Sprintf("validator lock denom invariants:\n\t lock denom: %v"+
					"\n\tvalidator allow denom: %v\n",
					lock.LockedCoin.Denom, valAllowedDenom)
			}
		}

		var multiStakingUnlocks []types.MultiStakingUnlock
		k.MultiStakingUnlockIterator(ctx, func(stakingUnlock types.MultiStakingUnlock) bool {
			multiStakingUnlocks = append(multiStakingUnlocks, stakingUnlock)
			return false
		})

		for _, unlock := range multiStakingUnlocks {
			valAddr := unlock.UnlockID.ValAddr
			valAllowedDenom := k.GetValidatorMultiStakingCoin(ctx, sdk.ValAddress(valAddr))

			for _, entry := range unlock.Entries {
				if entry.UnlockingCoin.Denom != valAllowedDenom {
					broken = true
					msg += fmt.Sprintf("validator unlock denom invariants:\n\t unlock denom: %v"+
						"\n\tvalidator allow denom: %v\n"+
						"\n\t entry height %v"+
						"\n\t validator address %s deladdress %s",
						entry.UnlockingCoin.Denom, valAllowedDenom, entry.CreationHeight, unlock.UnlockID.ValAddr, unlock.UnlockID.MultiStakerAddr)
				}
			}
		}

		return sdk.FormatInvariant(types.ModuleName, "validator lock denom", fmt.Sprintf("found invalid validator lock denom\n%s", msg)), broken
	}
}
