package api

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	sw "github.com/HydroProtocol/hydro-scaffold-dex/backend/sdk_wrappers"
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/utils" // For DecimalToBigInt, Dump
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"strings"
	// "github.com/HydroProtocol/hydro-sdk-go/sdk" // Placeholder for actual SDK package
)

// BorrowLoan handles the request to borrow an asset in a margin market.
// It reuses CollateralManagementReq for simplicity as the request body is similar (marketID, assetAddress, amount).
func BorrowLoan(p Param) (interface{}, error) {
	req := p.(*CollateralManagementReq) // Reusing CollateralManagementReq

	// TODO: Get userAddress from authenticated context c.Get("userID").(string)
	reqUserAddress := req.GetAddress() // Relies on BaseReq.Address being set by auth or previous logic
	if reqUserAddress == "" {
		// Fallback for environments where auth might not inject address for POSTs yet
		reqUserAddress = "0xSIMULATEDUSERADDRESSFORPOST" // Placeholder
		// return nil, NewApiError(http.StatusUnauthorized, "User address not found in authenticated context")
	}
	commonUserAddress := common.HexToAddress(reqUserAddress)

	if !req.Amount.IsPositive() {
		return nil, NewApiError(http.StatusBadRequest, "Amount to borrow must be positive")
	}

	market := models.MarketDao.FindMarketByID(req.MarketID)
	if market == nil {
		return nil, NewApiError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	if !market.BorrowEnable {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Margin trading (borrowing) not enabled for market %s", req.MarketID))
	}

	if req.AssetAddress != market.BaseTokenAddress && req.AssetAddress != market.QuoteTokenAddress {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid asset address %s for market %s. Must be base or quote token to borrow.", req.AssetAddress, req.MarketID))
	}

	var tokenDecimals int
	if req.AssetAddress == market.BaseTokenAddress {
		tokenDecimals = market.BaseTokenDecimals
	} else {
		tokenDecimals = market.QuoteTokenDecimals
	}

	// --- Pre-Borrow Collateral Check (Placeholder) ---
	commonAssetAddress := common.HexToAddress(req.AssetAddress)
	uint16MarketID, err := sw.MarketIDToUint16(req.MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID format for SDK: %v", err))
	}

	// --- Pre-Borrow Collateral Check ---
	currentAccountDetails, err := sw.GetAccountDetails(hydro, commonUserAddress, uint16MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to get current account details for pre-borrow check: %v", err))
	}
	currentAssetsUSD := utils.WeiToDecimalAmount(currentAccountDetails.AssetsTotalUSDValue, 18)
	currentDebtsUSD := utils.WeiToDecimalAmount(currentAccountDetails.DebtsTotalUSDValue, 18)

	// Conceptual: Get price of the asset to be borrowed in USD. This requires an oracle or price feed.
	// For placeholder, assume 1 token = 1 USD for simplicity.
	priceOfBorrowedAssetUSD := decimal.NewFromInt(1) // Placeholder
	newLoanUSDValue := req.Amount.Mul(priceOfBorrowedAssetUSD)
	projectedTotalDebtsUSD := currentDebtsUSD.Add(newLoanUSDValue)

	utils.Dump("Pre-Borrow Check: CurrentAssetsUSD:", currentAssetsUSD.String(), "CurrentDebtsUSD:", currentDebtsUSD.String(), "NewLoanUSD:", newLoanUSDValue.String(), "ProjectedTotalDebtsUSD:", projectedTotalDebtsUSD.String(), "Market LiquidateRate:", market.LiquidateRate.String())

	if projectedTotalDebtsUSD.IsPositive() && currentAssetsUSD.LessThan(projectedTotalDebtsUSD.Mul(market.LiquidateRate)) {
		// CRITICAL: market.LiquidateRate should be > 1 for this math (e.g. 1.5 for 150% collateralization)
		// Or, if LiquidateRate is < 1 (e.g. 0.75 for 75% LTV), then check: currentAssetsUSD * market.LiquidateRate < projectedTotalDebtsUSD
		// Assuming LiquidateRate is like a collateral ratio (e.g., 1.1 means assets must be 110% of debts)
		// So, if Assets < Debts * 1.1, it's not allowed.
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Borrowing this amount would bring collateral ratio below liquidation threshold. Assets: %s, Projected Debts: %s, Required Ratio: %s", currentAssetsUSD.String(), projectedTotalDebtsUSD.String(), market.LiquidateRate.String()))
	}
	// Placeholder for GetInterestRates - not directly used in pre-borrow check logic here yet, but might be for other validations.
	_, err = sw.GetInterestRates(hydro, commonAssetAddress, utils.DecimalToWei(req.Amount, tokenDecimals))
	if err != nil {
		utils.Warning("Failed to get interest rates during pre-borrow check (non-critical for this check): %v", err)
	}
	utils.Dump("Pre-borrow collateral check passed.")

	// --- Construct Batch Action ---
	amountBigInt := utils.DecimalToWei(req.Amount, tokenDecimals)
	encodedParams, err := sw.EncodeBorrowParamsForBatch(uint16MarketID, commonAssetAddress, amountBigInt)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to encode borrow params: %v", err))
	}

	action := sw.SDKBatchAction{
		ActionType:    sw.SDKActionTypeBorrow,
		EncodedParams: encodedParams,
	}

	// --- Prepare Unsigned Transaction ---
	if sw.MarginContractAddress == (common.Address{}) {
		return nil, NewError(http.StatusInternalServerError, "Margin contract address not initialized")
	}
	marginContractABIForPack, err := abi.JSON(strings.NewReader(sw.MarginContractABIJsonString))
	if err != nil {
		return nil, NewError(http.StatusInternalServerError, "Failed to parse margin contract ABI for packing")
	}
	packedBatchData, err := marginContractABIForPack.Pack("batch", []sw.SDKBatchAction{action})
	if err != nil {
		return nil, NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to pack batch actions for borrow: %v", err))
	}

	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     commonUserAddress.Hex(),
		To:       sw.MarginContractAddress.Hex(),
		Value:    "0", // Assuming no ETH value sent directly to batch function
		Data:     common.Bytes2Hex(packedBatchData),
		GasPrice: "0", // Placeholder - should be estimated or from config
		GasLimit: "0", // Placeholder - should be estimated
	}
	utils.Info("Prepared unsigned transaction for borrow loan.")
	return unsignedTxForClient, nil
}

// GetLoans lists the current loans for a user in a specific margin market.
func GetLoans(p Param) (interface{}, error) {
	req := p.(*LoanListReq)

	// TODO: Replace with userAddress from authenticated context c.Get("userID").(string)
	// once LoanListReq stops taking UserAddress as a query parameter and relies on auth.
	reqUserAddress := req.GetAddress() // Uses LoanListReq.GetAddress()
	if reqUserAddress == "" {
		// This case should ideally be prevented by validation on LoanListReq.UserAddress or caught by auth middleware.
		return nil, NewApiError(http.StatusBadRequest, "User address is required")
	}
	commonUserAddress := common.HexToAddress(reqUserAddress)

	market := models.MarketDao.FindMarketByID(req.MarketID)
	if market == nil {
		return nil, NewApiError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	if !market.BorrowEnable { // Only margin-enabled markets can have loans
		// Return empty list for non-margin markets, or an error, depending on desired behavior.
		// For now, let's assume if not borrow enabled, no loans are possible.
		return LoanListResp{MarketID: req.MarketID, UserAddress: userAddress, Loans: []LoanDetails{}}, nil
	}

	loans := []LoanDetails{}

	// --- Fetch Base Token Borrowed Amount (SDK Call Placeholder) ---
	uint16MarketID, err := sw.MarketIDToUint16(req.MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID format for SDK: %v", err))
	}

	loans := []LoanDetails{}
	commonBaseTokenAddress := common.HexToAddress(market.BaseTokenAddress)
	commonQuoteTokenAddress := common.HexToAddress(market.QuoteTokenAddress)

	// --- Fetch Base Token Borrowed Amount ---
	baseBorrowedBigInt, err := sw.GetAmountBorrowed(hydro, commonUserAddress, uint16MarketID, commonBaseTokenAddress)
	if err != nil {
		utils.Errorf("Failed to get base token borrowed amount for user %s, market %s: %v", reqUserAddress, req.MarketID, err)
		// Decide if this is a fatal error or if we can continue to fetch quote token loan
		// return nil, NewApiError(http.StatusInternalServerError, "Failed to fetch base token loan details")
	} else {
		if baseBorrowedBigInt.Cmp(big.NewInt(0)) > 0 {
			baseBorrowedDecimal := utils.WeiToDecimalAmount(baseBorrowedBigInt, market.BaseTokenDecimals)
			baseBorrowedDecimal := utils.WeiToDecimalAmount(baseBorrowedBigInt, market.BaseTokenDecimals)
			loanDetail := LoanDetails{
				AssetAddress:   market.BaseTokenAddress,
				Symbol:         market.BaseTokenSymbol,
				AmountBorrowed: baseBorrowedDecimal,
			}

			// Optionally, fetch and add interest rates
			// Note: GetInterestRates might take an `extraBorrowAmount` which is 0 for current rates.
			// The actual calculation of accrued interest might be complex and depend on time, etc.
			// For now, just fetching the rate.
			interestRates, errRates := sw.GetInterestRates(hydro, commonBaseTokenAddress, big.NewInt(0))
			if errRates != nil {
				utils.Warningf("Failed to get interest rates for base asset %s in market %s: %v", market.BaseTokenSymbol, req.MarketID, errRates)
				// Do not fail the whole request, just omit interest rate if not available
			} else if interestRates != nil {
				// Assuming BorrowInterestRate is an annual rate, needs conversion based on contract's representation (e.g., per block, per second)
				// This is a placeholder for actual accrued interest calculation.
				// For now, storing the raw rate as a decimal (assuming 18 decimals for the rate itself).
				loanDetail.CurrentInterestRate = utils.WeiToDecimalAmount(interestRates.BorrowInterestRate, 18) // Placeholder: use actual rate decimals
				utils.Infof("Interest rate for base asset %s: %s", market.BaseTokenSymbol, loanDetail.CurrentInterestRate.String())
			}
			loans = append(loans, loanDetail)
		}
	}

	// --- Fetch Quote Token Borrowed Amount ---
	quoteBorrowedBigInt, err := sw.GetAmountBorrowed(hydro, commonUserAddress, uint16MarketID, commonQuoteTokenAddress)
	if err != nil {
		utils.Errorf("Failed to get quote token borrowed amount for user %s, market %s: %v", reqUserAddress, req.MarketID, err)
		// return nil, NewApiError(http.StatusInternalServerError, "Failed to fetch quote token loan details")
	} else {
		if quoteBorrowedBigInt.Cmp(big.NewInt(0)) > 0 {
			quoteBorrowedDecimal := utils.WeiToDecimalAmount(quoteBorrowedBigInt, market.QuoteTokenDecimals)
			loanDetail := LoanDetails{
				AssetAddress:   market.QuoteTokenAddress,
				Symbol:         market.QuoteTokenSymbol,
				AmountBorrowed: quoteBorrowedDecimal,
			}

			interestRates, errRates := sw.GetInterestRates(hydro, commonQuoteTokenAddress, big.NewInt(0))
			if errRates != nil {
				utils.Warningf("Failed to get interest rates for quote asset %s in market %s: %v", market.QuoteTokenSymbol, req.MarketID, errRates)
			} else if interestRates != nil {
				loanDetail.CurrentInterestRate = utils.WeiToDecimalAmount(interestRates.BorrowInterestRate, 18) // Placeholder
				utils.Infof("Interest rate for quote asset %s: %s", market.QuoteTokenSymbol, loanDetail.CurrentInterestRate.String())
			}
			loans = append(loans, loanDetail)
		}
	}

	return LoanListResp{MarketID: req.MarketID, UserAddress: reqUserAddress, Loans: loans}, nil
}

// RepayLoan handles the request to repay a borrowed asset in a margin market.
// It reuses CollateralManagementReq for simplicity.
func RepayLoan(p Param) (interface{}, error) {
	req := p.(*CollateralManagementReq) // Reusing CollateralManagementReq

	// TODO: Get userAddress from authenticated context c.Get("userID").(string)
	reqUserAddress := req.GetAddress()
	if reqUserAddress == "" {
		reqUserAddress = "0xSIMULATEDUSERADDRESSFORPOST" // Placeholder
	}
	commonUserAddress := common.HexToAddress(reqUserAddress)

	if !req.Amount.IsPositive() {
		// Note: Some systems might allow repaying with 0 or a special value like MaxUint256 to repay all.
		return nil, NewApiError(http.StatusBadRequest, "Amount to repay must be positive")
	}

	market := models.MarketDao.FindMarketByID(req.MarketID)
	if market == nil {
		return nil, NewApiError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	// BorrowEnable check might be redundant for repay, but market must exist and generally be a margin market.
	if !market.BorrowEnable {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Margin trading (repaying) not enabled for market %s", req.MarketID))
	}

	if req.AssetAddress != market.BaseTokenAddress && req.AssetAddress != market.QuoteTokenAddress {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid asset address %s for market %s. Must be base or quote token to repay.", req.AssetAddress, req.MarketID))
	}

	var tokenDecimals int
	if req.AssetAddress == market.BaseTokenAddress {
		tokenDecimals = market.BaseTokenDecimals
	} else {
		tokenDecimals = market.QuoteTokenDecimals
	}

	// --- Check Collateral Balance for Repayment (Placeholder) ---
	// Repayments are made FROM the collateral deposited in the margin account for that specific asset.
	commonAssetAddress := common.HexToAddress(req.AssetAddress)
	uint16MarketID, err := sw.MarketIDToUint16(req.MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID format for SDK: %v", err))
	}

	// --- Check Collateral Balance for Repayment ---
	// Repayments are made FROM the collateral deposited in the margin account for that specific asset.
	collateralBalanceBigInt, err := sw.MarketBalanceOf(hydro, uint16MarketID, commonAssetAddress, commonUserAddress)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to get collateral balance for asset %s: %v", req.AssetAddress, err))
	}
	collateralBalance := utils.WeiToDecimalAmount(collateralBalanceBigInt, tokenDecimals)
	if req.Amount.GreaterThan(collateralBalance) {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Insufficient collateral balance of %s to repay %s. Available: %s",
			market.GetTokenSymbolByAddress(req.AssetAddress), req.Amount.String(), collateralBalance.String()))
	}
	utils.Dump("Collateral balance check for repayment passed. User:", reqUserAddress, "Market:", req.MarketID, "Asset:", req.AssetAddress, "Amount:", req.Amount.String())

	// --- Construct Batch Action ---
	amountBigInt := utils.DecimalToWei(req.Amount, tokenDecimals)
	encodedParams, err := sw.EncodeRepayParamsForBatch(uint16MarketID, commonAssetAddress, amountBigInt)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to encode repay params: %v", err))
	}

	action := sw.SDKBatchAction{
		ActionType:    sw.SDKActionTypeRepay,
		EncodedParams: encodedParams,
	}

	// --- Prepare Unsigned Transaction ---
	if sw.MarginContractAddress == (common.Address{}) {
		return nil, NewError(http.StatusInternalServerError, "Margin contract address not initialized")
	}
	marginContractABIForPack, err := abi.JSON(strings.NewReader(sw.MarginContractABIJsonString))
	if err != nil {
		return nil, NewError(http.StatusInternalServerError, "Failed to parse margin contract ABI for packing")
	}
	packedBatchData, err := marginContractABIForPack.Pack("batch", []sw.SDKBatchAction{action})
	if err != nil {
		return nil, NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to pack batch actions for repay: %v", err))
	}

	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     commonUserAddress.Hex(),
		To:       sw.MarginContractAddress.Hex(),
		Value:    "0", // Assuming no ETH value sent directly to batch function
		Data:     common.Bytes2Hex(packedBatchData),
		GasPrice: "0", // Placeholder - should be estimated or from config
		GasLimit: "0", // Placeholder - should be estimated
	}
	utils.Info("Prepared unsigned transaction for repay loan.")
	return unsignedTxForClient, nil
}
