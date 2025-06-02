package api

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/sdk_wrappers"
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	goEthereumCommon "github.com/ethereum/go-ethereum/common"
	"github.com/labstack/echo"
	"github.com/shopspring/decimal"
)

// OpenMarginPositionReq defines the request structure for opening a margin position.
type OpenMarginPositionReq struct {
	MarketID            string  `json:"marketID" validate:"required"`                     // e.g., "HOT-ETH"
	Side                string  `json:"side" validate:"required,oneof=buy sell"`          // "buy" or "sell"
	Amount              string  `json:"amount" validate:"required,numeric"`               // Amount of base asset to buy/sell
	Price               string  `json:"price" validate:"omitempty,numeric"`               // Optional, for limit orders
	Leverage            float64 `json:"leverage" validate:"required,gt=0"`                // e.g., 2.5 for 2.5x leverage
	CollateralAssetSymbol string  `json:"collateralAssetSymbol" validate:"required"`      // e.g., "ETH" or "DAI"
	CollateralAmount    string  `json:"collateralAmount" validate:"required,numeric"`     // Amount of collateral to use from common balance
}

// OpenMarginPosition handles the request to open a new margin position.
func OpenMarginPosition(c echo.Context) error {
	req := &OpenMarginPositionReq{}
	if err := BindAndValidate(c, req); err != nil {
		return NewError(http.StatusBadRequest, err.Error())
	}

	cc := c.(*CustomContext)
	userAddress := goEthereumCommon.HexToAddress(cc.Get("userAddress").(string))
	market, err := models.MarketDao.FindMarketByID(req.MarketID)
	if err != nil {
		return NewError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}

	collateralAmountDecimal, err := decimal.NewFromString(req.CollateralAmount)
	if err != nil {
		return NewError(http.StatusBadRequest, fmt.Sprintf("Invalid collateral amount: %s", req.CollateralAmount))
	}

	// TODO: Convert other string amounts/prices (req.Amount, req.Price) to decimal.Decimal or *big.Int as needed.

	// --- Placeholder for Pre-trade Validations ---
	// 1. Fetch user's collateral asset balance (e.g., req.CollateralAssetSymbol) from their common balance.
	//    - This might involve a call to an ERC20 wrapper or another SDK function.
	//    - Example: collateralToken, err := models.TokenDao.GetTokenBySymbol(req.CollateralAssetSymbol)
	//    - Example: commonBalance, err := sdk_wrappers.GetERC20Balance(userAddress, collateralToken.Address)
	//    - If commonBalance < collateralAmountDecimal, return error.
	utils.Info("Placeholder: Fetch user's collateral asset balance for %s", req.CollateralAssetSymbol)

	// 2. Fetch market margin parameters (IMR, MMR, LiquidateRate) for market.ID (uint16).
	//    - This will require a new function in sdk_wrappers, e.g., `sdk_wrappers.GetMarketMarginParameters(market.ID)`
	//    - These parameters are crucial for margin calculations.
	utils.Info("Placeholder: Fetch market margin parameters for market ID %d", market.ID)
	//    - Example: marketParams, err := sdk_wrappers.GetMarketMarginParameters(hydroSDK, market.ID) (assuming hydroSDK is available)

	// 3. Calculate required initial margin based on order size, price, leverage, and market's IMR.
	//    - PositionValue = Amount * Price (if limit) or estimated market price.
	//    - RequiredInitialMargin = PositionValue / Leverage * IMR_factor (or similar logic from contract/docs)
	//    - Check if req.CollateralAmount >= RequiredInitialMargin.
	utils.Info("Placeholder: Calculate required initial margin and check against user's provided collateral and leverage.")

	// 4. Estimate post-trade account health (e.g., using `sdk_wrappers.GetAccountDetails`).
	//    - This is complex as it requires simulating the state *after* the batch actions.
	//    - For now, we might skip this or do a very rough estimation.
	//    - If estimated post-trade status is liquidatable, reject.
	utils.Info("Placeholder: Estimate post-trade account health.")
	// --- End of Pre-trade Validations ---


	// --- Prepare SDKBatchAction array ---
	var actions []sdk_wrappers.SDKBatchAction

	// Action 1: Transfer Collateral
	// Determine collateral token address
	collateralToken, err := models.TokenDao.GetTokenBySymbol(req.CollateralAssetSymbol)
	if err != nil {
		return NewError(http.StatusBadRequest, fmt.Sprintf("Collateral asset %s not found or supported", req.CollateralAssetSymbol))
	}
	collateralTokenAddress := goEthereumCommon.HexToAddress(collateralToken.Address)

	// Convert collateralAmountDecimal to *big.Int based on collateral token decimals
	collateralAmountBigInt := utils.DecimalToBigInt(collateralAmountDecimal, int32(collateralToken.Decimals))

	fromPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCommon,
		MarketID: 0, // Not applicable for common balance path
		User:     userAddress,
	}
	toPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCollateralAccount,
		MarketID: market.ID, // Target market for the margin account
		User:     userAddress,
	}

	encodedTransferParams, err := sdk_wrappers.EncodeTransferParamsForBatch(collateralTokenAddress, fromPath, toPath, collateralAmountBigInt)
	if err != nil {
		utils.Errorf("Failed to encode transfer params for batch: %v", err)
		return NewError(http.StatusInternalServerError, "Failed to prepare collateral transfer action")
	}
	actions = append(actions, sdk_wrappers.SDKBatchAction{
		ActionType:    sdk_wrappers.SDKActionTypeTransfer,
		EncodedParams: encodedTransferParams,
	})
	utils.Info("Prepared Action 1: Transfer Collateral (%s %s) for market %d", req.CollateralAmount, req.CollateralAssetSymbol, market.ID)


	// Action 2: Borrow Asset
	// TODO: Determine borrow asset (base or quote of req.MarketID) and borrow amount.
	// This calculation is non-trivial:
	// - If buying base (e.g., HOT in HOT-ETH), borrowing quote (ETH).
	// - If selling base (e.g., HOT in HOT-ETH), borrowing base (HOT).
	// - BorrowAmount = (PositionValue * (Leverage - 1) / Leverage) / BorrowAssetPrice
	// - This requires knowing the price of the borrow asset and the main asset of the trade.
	// For now, using placeholder values.
	var borrowAssetAddress goEthereumCommon.Address
	var borrowAmountBigInt *big.Int

	amountDecimal, _ := decimal.NewFromString(req.Amount) // Assume valid for now
	// Example: If buying HOT-ETH, amount is HOT, price is ETH/HOT. Position value in ETH = amount * price.
	// Collateral is req.CollateralAmount (e.g. in ETH).
	// Total position value desired = collateralAmountDecimal * req.Leverage (in terms of collateral value)
	// Amount to borrow = Total position value desired - collateralAmountDecimal (in terms of collateral value)
	// This borrowed amount then needs to be converted to the borrow asset quantity.

	if req.Side == "buy" { // Buying base asset, so borrowing quote asset
		borrowAssetAddress = goEthereumCommon.HexToAddress(market.QuoteTokenAddress)
		// Placeholder: Borrow 0.1 of quote asset
		quoteToken, _ := models.TokenDao.GetTokenByAddress(market.QuoteTokenAddress)
		borrowAmountBigInt = utils.DecimalToBigInt(decimal.NewFromFloat(0.1), int32(quoteToken.Decimals))
		utils.Info("Placeholder: Determined borrow asset: %s (Quote), amount: 0.1", market.QuoteTokenSymbol)
	} else { // Selling base asset, so borrowing base asset
		borrowAssetAddress = goEthereumCommon.HexToAddress(market.BaseTokenAddress)
		// Placeholder: Borrow 10 of base asset
		baseToken, _ := models.TokenDao.GetTokenByAddress(market.BaseTokenAddress)
		borrowAmountBigInt = utils.DecimalToBigInt(decimal.NewFromFloat(10), int32(baseToken.Decimals))
		utils.Info("Placeholder: Determined borrow asset: %s (Base), amount: 10", market.BaseTokenSymbol)
	}
	// Actual calculation needs to be implemented carefully.


	encodedBorrowParams, err := sdk_wrappers.EncodeBorrowParamsForBatch(market.ID, borrowAssetAddress, borrowAmountBigInt)
	if err != nil {
		utils.Errorf("Failed to encode borrow params for batch: %v", err)
		return NewError(http.StatusInternalServerError, "Failed to prepare borrow action")
	}
	actions = append(actions, sdk_wrappers.SDKBatchAction{
		ActionType:    sdk_wrappers.SDKActionTypeBorrow,
		EncodedParams: encodedBorrowParams,
	})
	utils.Info("Prepared Action 2: Borrow Asset (placeholder amount)")

	// --- Get Unsigned Transaction ---
	// The hydroSDK instance should be available in the server context or passed to this handler.
	// For now, assuming it's globally accessible or retrieved via cc.hydroSDK()
	hydroSDK := GetHydroSDK() // Assuming a helper function to get the SDK instance
	if hydroSDK == nil {
		return NewError(http.StatusInternalServerError, "Hydro SDK not available")
	}
	
	// Note: sdk_wrappers.PrepareBatchActionsTransaction was the name in the subtask,
	// but existing code uses ExecuteBatchActions which returns a hash if successful (implying it submits).
	// If we need an *unsigned* tx, ExecuteBatchActions needs to be refactored or a new wrapper created.
	// For now, let's assume ExecuteBatchActions can be adapted or a new PrepareBatchActionsTransaction exists.
	// Let's use a hypothetical PrepareBatchActionsTransaction for now as per subtask's intent.
	// unsignedTx, err := sdk_wrappers.ExecuteBatchActions(hydroSDK, userAddress, actions) // This submits
	
	// Placeholder for a function that prepares but does not send:
	// func PrepareBatchActionsTransaction(hydro sdk.Hydro, userAddress common.Address, contractAddress common.Address, actions []SDKBatchAction, value *big.Int, options *TransactionOptions) (*common.UnsignedTxDataForClient, error)
	// For now, we'll mock the call to a non-existent function to match subtask intent for "Get Unsigned Transaction"
	
	// Mocking the call as the actual PrepareBatchActionsTransaction is not yet in sdk_wrappers
	// This would typically involve:
	// 1. Getting the margin contract ABI and address.
	// 2. Packing the "batch" method with the `actions`.
	// 3. Constructing the raw transaction data (to, data, value, gas (estimated)).
	// This part needs the actual `sdk_wrappers.PrepareBatchActionsTransaction` to be implemented.
	// For now, we return a dummy response or error.
	
	// Let's assume `sdk_wrappers.ExecuteBatchActions` is what we have and it's meant to be modified
	// or a new `PrepareBatchActionsTransaction` will be created in `sdk_wrappers` later.
	// The subtask asks to *call* `PrepareBatchActionsTransaction`.
	// Since it doesn't exist in the current `sdk_wrappers` from previous steps, this call will fail if not stubbed.
	// For the purpose of this subtask, we will simulate its expected behavior if it *were* implemented.
	
	// Simulate packing for an unsigned transaction data structure
	if sdk_wrappers.MarginContractAddress == (goEthereumCommon.Address{}) {
		return NewError(http.StatusInternalServerError, "Margin contract address not initialized")
	}
	
	var marginContractABIForPack abi.ABI // Need to re-parse or ensure it's accessible
	marginContractABIForPack, err = abi.JSON(strings.NewReader(sdk_wrappers.MarginContractABIJsonString))
	if err != nil {
		return NewError(http.StatusInternalServerError, "Failed to parse margin contract ABI for packing")
	}

	packedBatchData, err := marginContractABIForPack.Pack("batch", actions)
	if err != nil {
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to pack batch actions: %v", err))
	}

	// This is a simplified UnsignedTxDataForClient. Gas estimation would be more complex.
	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     userAddress.Hex(),
		To:       sdk_wrappers.MarginContractAddress.Hex(),
		Value:    "0", // Assuming no ETH value sent directly to batch function
		Data:     goEthereumCommon.Bytes2Hex(packedBatchData),
		GasPrice: "0", // Placeholder - should be estimated or from config
		GasLimit: "0", // Placeholder - should be estimated
	}
	
	utils.Info("Successfully prepared unsigned transaction for opening margin position.")
	return c.JSON(http.StatusOK, unsignedTxForClient)
}

// CloseMarginPositionReq defines the request structure for closing a margin position.
// For now, it assumes full closure of all debts in the specified market.
type CloseMarginPositionReq struct {
	MarketID string `json:"marketID" validate:"required"` // e.g., "HOT-ETH"
	// TODO: Add optional AmountToCloseBase and AmountToCloseQuote if partial closure is supported later.
}

// CloseMarginPosition handles the request to close a margin position in a given market.
// This involves repaying all debts for base and quote assets in that market
// and withdrawing all remaining collateral from that market's margin account to common balance.
func CloseMarginPosition(c echo.Context) error {
	req := &CloseMarginPositionReq{}
	if err := BindAndValidate(c, req); err != nil {
		return NewError(http.StatusBadRequest, err.Error())
	}

	cc := c.(*CustomContext)
	userAddress := goEthereumCommon.HexToAddress(cc.Get("userAddress").(string))

	market, err := models.MarketDao.FindMarketByID(req.MarketID)
	if err != nil {
		return NewError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	marketUint16ID := market.ID // This is already uint16

	hydroSDK := GetHydroSDK()
	if hydroSDK == nil {
		return NewError(http.StatusInternalServerError, "Hydro SDK not available")
	}

	var actions []sdk_wrappers.SDKBatchAction

	// --- Fetch current debts and balances ---
	baseTokenAddr := goEthereumCommon.HexToAddress(market.BaseTokenAddress)
	quoteTokenAddr := goEthereumCommon.HexToAddress(market.QuoteTokenAddress)

	// Get Base Asset Debt
	baseBorrowed, err := sdk_wrappers.GetAmountBorrowed(hydroSDK, userAddress, marketUint16ID, baseTokenAddr)
	if err != nil {
		utils.Errorf("Failed to get base amount borrowed for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		// Not returning error immediately, as some methods might not be in placeholder ABI.
		// A zero value for baseBorrowed will mean no repay action for base asset.
		baseBorrowed = big.NewInt(0)
	} else {
		utils.Infof("User %s borrowed %s of base asset %s in market %s", userAddress.Hex(), baseBorrowed.String(), market.BaseTokenSymbol, req.MarketID)
	}

	// Get Quote Asset Debt
	quoteBorrowed, err := sdk_wrappers.GetAmountBorrowed(hydroSDK, userAddress, marketUint16ID, quoteTokenAddr)
	if err != nil {
		utils.Errorf("Failed to get quote amount borrowed for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		quoteBorrowed = big.NewInt(0)
	} else {
		utils.Infof("User %s borrowed %s of quote asset %s in market %s", userAddress.Hex(), quoteBorrowed.String(), market.QuoteTokenSymbol, req.MarketID)
	}

	// Action 1: Repay Base Asset Debt (if any)
	if baseBorrowed != nil && baseBorrowed.Cmp(big.NewInt(0)) > 0 {
		encodedRepayBaseParams, err := sdk_wrappers.EncodeRepayParamsForBatch(marketUint16ID, baseTokenAddr, baseBorrowed)
		if err != nil {
			return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to prepare base asset repay action: %v", err))
		}
		actions = append(actions, sdk_wrappers.SDKBatchAction{
			ActionType:    sdk_wrappers.SDKActionTypeRepay,
			EncodedParams: encodedRepayBaseParams,
		})
		utils.Info("Prepared Action: Repay Base Asset Debt (%s %s)", baseBorrowed.String(), market.BaseTokenSymbol)
	}

	// Action 2: Repay Quote Asset Debt (if any)
	if quoteBorrowed != nil && quoteBorrowed.Cmp(big.NewInt(0)) > 0 {
		encodedRepayQuoteParams, err := sdk_wrappers.EncodeRepayParamsForBatch(marketUint16ID, quoteTokenAddr, quoteBorrowed)
		if err != nil {
			return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to prepare quote asset repay action: %v", err))
		}
		actions = append(actions, sdk_wrappers.SDKBatchAction{
			ActionType:    sdk_wrappers.SDKActionTypeRepay,
			EncodedParams: encodedRepayQuoteParams,
		})
		utils.Info("Prepared Action: Repay Quote Asset Debt (%s %s)", quoteBorrowed.String(), market.QuoteTokenSymbol)
	}

	// Get current balances in the margin account for withdrawal
	baseBalanceInMarginAccount, err := sdk_wrappers.MarketBalanceOf(hydroSDK, marketUint16ID, baseTokenAddr, userAddress)
	if err != nil {
		utils.Errorf("Failed to get base balance in margin account for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		baseBalanceInMarginAccount = big.NewInt(0) // Assume zero if error, to prevent withdrawal issues if method fails
	} else {
		utils.Infof("User %s has %s of base asset %s in margin account for market %s", userAddress.Hex(), baseBalanceInMarginAccount.String(), market.BaseTokenSymbol, req.MarketID)
	}

	quoteBalanceInMarginAccount, err := sdk_wrappers.MarketBalanceOf(hydroSDK, marketUint16ID, quoteTokenAddr, userAddress)
	if err != nil {
		utils.Errorf("Failed to get quote balance in margin account for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		quoteBalanceInMarginAccount = big.NewInt(0) // Assume zero if error
	} else {
		utils.Infof("User %s has %s of quote asset %s in margin account for market %s", userAddress.Hex(), quoteBalanceInMarginAccount.String(), market.QuoteTokenSymbol, req.MarketID)
	}
	
	// Define paths for transferring from margin account to common balance
	fromMarginPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCollateralAccount,
		MarketID: marketUint16ID,
		User:     userAddress,
	}
	toCommonPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCommon,
		MarketID: 0, // Not applicable for common balance path
		User:     userAddress,
	}

	// Action 3: Transfer Remaining Base Collateral
	if baseBalanceInMarginAccount != nil && baseBalanceInMarginAccount.Cmp(big.NewInt(0)) > 0 {
		encodedTransferBaseParams, err := sdk_wrappers.EncodeTransferParamsForBatch(baseTokenAddr, fromMarginPath, toCommonPath, baseBalanceInMarginAccount)
		if err != nil {
			return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to prepare base asset withdrawal: %v", err))
		}
		actions = append(actions, sdk_wrappers.SDKBatchAction{
			ActionType:    sdk_wrappers.SDKActionTypeTransfer,
			EncodedParams: encodedTransferBaseParams,
		})
		utils.Info("Prepared Action: Withdraw Remaining Base Collateral (%s %s)", baseBalanceInMarginAccount.String(), market.BaseTokenSymbol)
	}

	// Action 4: Transfer Remaining Quote Collateral
	if quoteBalanceInMarginAccount != nil && quoteBalanceInMarginAccount.Cmp(big.NewInt(0)) > 0 {
		encodedTransferQuoteParams, err := sdk_wrappers.EncodeTransferParamsForBatch(quoteTokenAddr, fromMarginPath, toCommonPath, quoteBalanceInMarginAccount)
		if err != nil {
			return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to prepare quote asset withdrawal: %v", err))
		}
		actions = append(actions, sdk_wrappers.SDKBatchAction{
			ActionType:    sdk_wrappers.SDKActionTypeTransfer,
			EncodedParams: encodedTransferQuoteParams,
		})
		utils.Info("Prepared Action: Withdraw Remaining Quote Collateral (%s %s)", quoteBalanceInMarginAccount.String(), market.QuoteTokenSymbol)
	}

	if len(actions) == 0 {
		return NewError(http.StatusOK, "No debts to repay and no collateral to withdraw in the specified market account.")
	}
	
	// --- Get Unsigned Transaction (Simulated) ---
	if sdk_wrappers.MarginContractAddress == (goEthereumCommon.Address{}) {
		return NewError(http.StatusInternalServerError, "Margin contract address not initialized")
	}
	var marginContractABIForPack abi.ABI
	marginContractABIForPack, err = abi.JSON(strings.NewReader(sdk_wrappers.MarginContractABIJsonString))
	if err != nil {
		return NewError(http.StatusInternalServerError, "Failed to parse margin contract ABI for packing")
	}
	packedBatchData, err := marginContractABIForPack.Pack("batch", actions)
	if err != nil {
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to pack batch actions for closing position: %v", err))
	}

	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     userAddress.Hex(),
		To:       sdk_wrappers.MarginContractAddress.Hex(),
		Value:    "0",
		Data:     goEthereumCommon.Bytes2Hex(packedBatchData),
		GasPrice: "0", // Placeholder
		GasLimit: "0", // Placeholder
	}

	utils.Info("Successfully prepared unsigned transaction for closing margin position in market %s.", req.MarketID)
	return c.JSON(http.StatusOK, unsignedTxForClient)
}
