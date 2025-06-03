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
	"github.com/labstack/echo"
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

	market := models.MarketDaoSql.FindMarketByID(req.MarketID) // Assuming MarketDaoSql is the global DAO
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

	commonAssetAddress := common.HexToAddress(req.AssetAddress)
	uint16MarketID, err := sw.MarketIDToUint16(req.MarketID) // Ensure MarketIDToUint16 is robust or market.ID_uint16() is used
	if err != nil {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID format for SDK: %v", err))
	}
	hydroSDK := GetHydroSDK() // Ensure GetHydroSDK is available

	// --- Pre-Borrow Collateral Check ---
	currentAccountDetails, err := sw.GetAccountDetails(hydroSDK, commonUserAddress, uint16MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to get current account details for pre-borrow check: %v", err))
	}
	currentAssetsUSD := utils.BigIntToDecimal(currentAccountDetails.AssetsTotalUSDValue, 18) // Assuming 18d for USD values
	currentDebtsUSD := utils.BigIntToDecimal(currentAccountDetails.DebtsTotalUSDValue, 18)  // Assuming 18d for USD values

	// Get price of the asset to be borrowed in USD
	assetToBorrowPriceUSD, errPrice := getPriceUSD(hydroSDK, commonAssetAddress, market.GetTokenSymbolByAddress(req.AssetAddress))
	if errPrice != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to get price for asset %s: %v", req.AssetAddress, errPrice))
	}
	if assetToBorrowPriceUSD.IsZero() {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Oracle price for asset %s is zero. Cannot perform USD calculations for borrow check.", req.AssetAddress))
	}

	newLoanUSDValue := req.Amount.Mul(assetToBorrowPriceUSD)
	projectedTotalDebtsUSD := currentDebtsUSD.Add(newLoanUSDValue)

	utils.Dump("Pre-Borrow Check: CurrentAssetsUSD:", currentAssetsUSD.String(), "CurrentDebtsUSD:", currentDebtsUSD.String(), "NewLoanUSD:", newLoanUSDValue.String(), "ProjectedTotalDebtsUSD:", projectedTotalDebtsUSD.String(), "Market LiquidateRate:", market.LiquidateRate.String())

	if projectedTotalDebtsUSD.IsPositive() && market.LiquidateRate.IsPositive() && currentAssetsUSD.Div(projectedTotalDebtsUSD).LessThan(market.LiquidateRate) {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Borrowing this amount would bring collateral ratio below liquidation threshold. AssetsUSD: %s, ProjectedDebtsUSD: %s, Required Ratio: %s", currentAssetsUSD.StringFixed(2), projectedTotalDebtsUSD.StringFixed(2), market.LiquidateRate.StringFixed(2)))
	}

	// --- Construct Batch Action ---
	amountBigInt := utils.DecimalToBigInt(req.Amount, int32(tokenDecimals))
	encodedParams, err := sw.EncodeBorrowParamsForBatch(uint16MarketID, commonAssetAddress, amountBigInt)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to encode borrow params: %v", err))
	}

	action := sw.SDKBatchAction{
		ActionType:    sw.SDKActionTypeBorrow,
		EncodedParams: encodedParams,
	}
    txValue := big.NewInt(0)
	unsignedTxForClient, err := sw.PrepareBatchActionsTransaction(hydroSDK, []sw.SDKBatchAction{action}, commonUserAddress, txValue)
    if err != nil {
        utils.Errorf("BorrowLoan: Failed to prepare batch actions transaction data: %v", err)
        return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to prepare transaction data: %v", err))
    }

	utils.Info("Prepared unsigned transaction for borrow loan.")
	return unsignedTxForClient, nil
}


type GetLoansReq struct {
	MarketID             string `json:"marketID" query:"marketID"` // Optional
	IncludeInterestRates bool   `json:"includeInterestRates" query:"includeInterestRates"`
}

// LoanDetail defines the structure for a single loan entry.
type LoanDetail struct {
	MarketID        uint16 `json:"marketID"`
	MarketSymbol    string `json:"marketSymbol"`
	AssetAddress    string `json:"assetAddress"`
	AssetSymbol     string `json:"assetSymbol"`
	BorrowedAmount  string `json:"borrowedAmount"`  // Native asset decimals
	InterestRateAPY string `json:"interestRateAPY"` // Placeholder or raw rate
	AccruedInterest string `json:"accruedInterest"` // Placeholder: "N/A"
}

// GetLoans retrieves a list of active loans for the authenticated user.
func (h *HydroApi) GetLoans(c echo.Context) error {
	req := new(GetLoansReq)
	if err := c.Bind(req); err != nil {
		return NewError(http.StatusBadRequest, err.Error())
	}
	// No specific validation for GetLoansReq yet, beyond what Bind does.

	customCtx := c.(*CustomContext)
	userAddressHex := customCtx.Get("userAddress").(string)
	if userAddressHex == "" {
		return NewError(http.StatusUnauthorized, "User address not available. Authentication required.")
	}
	userAddressCommon := common.HexToAddress(userAddressHex)

	hydroSDK := GetHydroSDK()
	if hydroSDK == nil {
		return NewError(http.StatusInternalServerError, "Hydro SDK not available")
	}

	var marketsToCheck []*models.Market
	if req.MarketID != "" {
		marketID_uint16, errConv := sw.MarketIDToUint16(req.MarketID) // Use sdk_wrappers helper
		if errConv != nil {
			return NewError(http.StatusBadRequest, "Invalid Market ID format")
		}
		market, errDb := models.MarketDaoSql.FindMarketByMarketID(marketID_uint16)
		if errDb != nil {
			return NewError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
		}
		if market != nil && market.BorrowEnable {
			marketsToCheck = append(marketsToCheck, market)
		}
	} else {
		allMarkets, errDb := models.MarketDaoSql.GetAllMarkets(true) // true for borrow_enable = true
		if errDb != nil {
			utils.Errorf("GetLoans: Failed to fetch all borrow-enabled markets: %v", errDb)
			return NewError(http.StatusInternalServerError, "Failed to fetch markets")
		}
		marketsToCheck = allMarkets
	}

	if len(marketsToCheck) == 0 {
		return c.JSON(http.StatusOK, &common.Response{Status: 0, Data: []LoanDetail{}})
	}

	var results []LoanDetail

	for _, market := range marketsToCheck {
		marketID_uint16 := market.ID_uint16()
		baseAssetAddr := common.HexToAddress(market.BaseTokenAddress)
		quoteAssetAddr := common.HexToAddress(market.QuoteTokenAddress)

		// Check base asset borrow
		baseBorrowedBigInt, errBase := sw.GetAmountBorrowed(hydroSDK, baseAssetAddr, userAddressCommon, marketID_uint16)
		if errBase != nil {
			utils.Warningf("GetLoans: Failed to get base borrowed amount for market %s, user %s: %v. Assuming 0.", market.ID, userAddressHex, errBase)
			baseBorrowedBigInt = big.NewInt(0)
		}
		if baseBorrowedBigInt.Sign() > 0 {
			loanDetail := LoanDetail{
				MarketID:        marketID_uint16,
				MarketSymbol:    market.ID,
				AssetAddress:    market.BaseTokenAddress,
				AssetSymbol:     market.BaseTokenSymbol,
				BorrowedAmount:  utils.BigWeiToDecimal(baseBorrowedBigInt, int32(market.BaseTokenDecimals)).StringFixed(market.AmountPrecision),
				AccruedInterest: "N/A", // Placeholder
			}
			if req.IncludeInterestRates {
				rates, rateErr := sw.GetInterestRates(hydroSDK, baseAssetAddr, big.NewInt(0)) // extraBorrowAmount = 0 for current rate
				if rateErr == nil && rates != nil && rates.BorrowInterestRate != nil {
					// Placeholder APY calculation - assumes rate is per second and 1e18 scaled.
					// Real APY = (1 + rate_per_period)^periods_per_year - 1
					// If rates.BorrowInterestRate is per second (e.g., 1e18 * 0.00000000X)
					ratePerSecond := decimal.NewFromBigInt(rates.BorrowInterestRate, -18)
					secondsPerYear := decimal.NewFromInt(365 * 24 * 60 * 60)
					// Simple interest for APY: rate_per_second * seconds_in_year * 100
					apyFromRaw := ratePerSecond.Mul(secondsPerYear).Mul(decimal.NewFromInt(100))
					loanDetail.InterestRateAPY = apyFromRaw.StringFixed(2) + "%"
					// TODO: Verify actual contract rate period (per second vs per block) and scaling for accurate APY.
				} else {
					loanDetail.InterestRateAPY = "Error fetching rate"
					utils.Warningf("GetLoans: Failed to get interest rates for base asset %s in market %s: %v", market.BaseTokenSymbol, market.ID, rateErr)
				}
			} else {
				loanDetail.InterestRateAPY = "N/A (not requested)"
			}
			results = append(results, loanDetail)
		}

		// Check quote asset borrow
		quoteBorrowedBigInt, errQuote := sw.GetAmountBorrowed(hydroSDK, quoteAssetAddr, userAddressCommon, marketID_uint16)
		if errQuote != nil {
			utils.Warningf("GetLoans: Failed to get quote borrowed amount for market %s, user %s: %v. Assuming 0.", market.ID, userAddressHex, errQuote)
			quoteBorrowedBigInt = big.NewInt(0)
		}
		if quoteBorrowedBigInt.Sign() > 0 {
			loanDetail := LoanDetail{
				MarketID:        marketID_uint16,
				MarketSymbol:    market.ID,
				AssetAddress:    market.QuoteTokenAddress,
				AssetSymbol:     market.QuoteTokenSymbol,
				BorrowedAmount:  utils.BigWeiToDecimal(quoteBorrowedBigInt, int32(market.QuoteTokenDecimals)).StringFixed(market.PricePrecision), // Using PricePrecision for quote asset amount
				AccruedInterest: "N/A", // Placeholder
			}
			if req.IncludeInterestRates {
				rates, rateErr := sw.GetInterestRates(hydroSDK, quoteAssetAddr, big.NewInt(0))
				if rateErr == nil && rates != nil && rates.BorrowInterestRate != nil {
					ratePerSecond := decimal.NewFromBigInt(rates.BorrowInterestRate, -18)
					secondsPerYear := decimal.NewFromInt(365 * 24 * 60 * 60)
					apyFromRaw := ratePerSecond.Mul(secondsPerYear).Mul(decimal.NewFromInt(100))
					loanDetail.InterestRateAPY = apyFromRaw.StringFixed(2) + "%"
					// TODO: Verify actual contract rate period and scaling for accurate APY.
				} else {
					loanDetail.InterestRateAPY = "Error fetching rate"
					utils.Warningf("GetLoans: Failed to get interest rates for quote asset %s in market %s: %v", market.QuoteTokenSymbol, market.ID, rateErr)
				}
			} else {
				loanDetail.InterestRateAPY = "N/A (not requested)"
			}
			results = append(results, loanDetail)
		}
	}

	return c.JSON(http.StatusOK, &common.Response{
		Status: 0,
		Data:   results,
	})
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
