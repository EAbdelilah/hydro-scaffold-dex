package api

import (
	"fmt"
	"math/big"
	"net/http"
	"strings" 
	"time"   

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/sdk_wrappers"
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	goEthereumCommon "github.com/ethereum/go-ethereum/common"
	"github.com/labstack/echo"
	"github.com/shopspring/decimal"
)

// Helper function to get asset price in USD
func getPriceUSD(hydroSDK sdk.Hydro, assetAddress goEthereumCommon.Address, assetSymbol string) (decimal.Decimal, error) {
	assetInfo, err := sdk_wrappers.GetAsset(hydroSDK, assetAddress) 
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get asset contract info for %s (%s): %v", assetSymbol, assetAddress.Hex(), err)
	}
	if assetInfo.PriceOracle == (goEthereumCommon.Address{}) { 
		return decimal.Zero, fmt.Errorf("no price oracle configured for asset %s (%s)", assetSymbol, assetAddress.Hex())
	}

	priceBigInt, err := sdk_wrappers.GetOraclePrice(hydroSDK, assetInfo.PriceOracle, assetAddress)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get oracle price for %s (%s) from oracle %s: %v", assetSymbol, assetAddress.Hex(), assetInfo.PriceOracle.Hex(), err)
	}
	// Assuming oracle prices are returned with 18 decimals for USD value
	return utils.BigIntToDecimal(priceBigInt, 18), nil
}


// OpenMarginPositionReq defines the request structure for opening a margin position.
type OpenMarginPositionReq struct {
	MarketID            string  `json:"marketID" validate:"required"`                     
	Side                string  `json:"side" validate:"required,oneof=buy sell"`          
	Amount              string  `json:"amount" validate:"required,numeric"`               
	Price               string  `json:"price" validate:"required,numeric"`               
	Leverage            float64 `json:"leverage" validate:"required,gt=0"`                
	CollateralAssetSymbol string  `json:"collateralAssetSymbol" validate:"required"`      
	CollateralAmount    string  `json:"collateralAmount" validate:"required,numeric"`     
}

// OpenMarginPosition handles the request to open a new margin position.
func OpenMarginPosition(c echo.Context) error {
	req := &OpenMarginPositionReq{}
	if err := BindAndValidate(c, req); err != nil {
		return NewError(http.StatusBadRequest, err.Error())
	}

	cc := c.(*CustomContext)
	userAddress := goEthereumCommon.HexToAddress(cc.Get("userAddress").(string))
	marketFromDB, err := models.MarketDao.FindMarketByID(req.MarketID)
	if err != nil {
		return NewError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	if !marketFromDB.BorrowEnable {
		return NewError(http.StatusBadRequest, fmt.Sprintf("Margin trading is not enabled for market %s", req.MarketID))
	}

	hydroSDK := GetHydroSDK() 
	if hydroSDK == nil {
		return NewError(http.StatusInternalServerError, "Hydro SDK not available")
	}

	// --- Convert Request Strings to Decimal ---
	amountDecimal, err := decimal.NewFromString(req.Amount)
	if err != nil { return NewError(http.StatusBadRequest, fmt.Sprintf("Invalid amount: %s", req.Amount)) }
	
	priceDecimal, err := decimal.NewFromString(req.Price) 
	if err != nil { return NewError(http.StatusBadRequest, fmt.Sprintf("Invalid price: %s", req.Price)) }
	
	collateralAmountDecimal, err := decimal.NewFromString(req.CollateralAmount)
	if err != nil { return NewError(http.StatusBadRequest, fmt.Sprintf("Invalid collateralAmount: %s", req.CollateralAmount)) }
	
	leverageDecimal := decimal.NewFromFloat(req.Leverage)

	if amountDecimal.IsNegativeOrZero() || priceDecimal.IsNegativeOrZero() || collateralAmountDecimal.IsNegativeOrZero() {
		return NewError(http.StatusBadRequest, "Amount, price, and collateral amount must be positive.")
	}
	if leverageDecimal.LessThanOrEqual(decimal.NewFromInt(1)) { 
		return NewError(http.StatusBadRequest, "Leverage must be greater than 1.")
	}
	
	// --- Fetch Market Margin Parameters ---
	// Using DB market model for IMR/LiquidateRate.
	// sdk_wrappers.GetMarketMarginParameters is a placeholder and would be preferred if it fetched live on-chain data.
	marketParams, err := sdk_wrappers.GetMarketMarginParameters(hydroSDK, marketFromDB.ID_uint16())
	if err != nil {
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to get market margin parameters for %s: %v", marketFromDB.ID, err))
	}
	// Override with DB values if they are considered more authoritative or if SDK returns defaults
	imrFraction_dec := marketFromDB.InitialMarginFraction 
	liquidateRate_dec := marketFromDB.LiquidateRate
	if marketFromDB.InitialMarginFraction.IsZero() { // If DB value is not set, use the (mocked) SDK one
	    imrFraction_dec = marketParams.InitialMarginFraction
	}
    if marketFromDB.LiquidateRate.IsZero() { // If DB value is not set, use the (mocked) SDK one
	    liquidateRate_dec = marketParams.LiquidateRate
	}

	if imrFraction_dec.LessThanOrEqual(decimal.Zero) || imrFraction_dec.GreaterThanOrEqual(decimal.NewFromInt(1)) {
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Market %s has invalid InitialMarginFraction: %s", marketFromDB.ID, imrFraction_dec.String()))
	}
	if liquidateRate_dec.LessThanOrEqual(decimal.NewFromInt(1)) { 
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Market %s has invalid LiquidateRate: %s", marketFromDB.ID, liquidateRate_dec.String()))
	}
	

	// --- Fetch User's Collateral Asset Balance (Common Wallet) ---
	collateralToken, err := models.TokenDao.GetTokenBySymbol(req.CollateralAssetSymbol)
	if err != nil {
		return NewError(http.StatusBadRequest, fmt.Sprintf("Collateral asset %s not found or supported", req.CollateralAssetSymbol))
	}
	collateralAssetAddress := goEthereumCommon.HexToAddress(collateralToken.Address)
	
	userCollateralAvailableInWallet, err := sdk_wrappers.BalanceOf(hydroSDK, collateralAssetAddress, userAddress)
	if err != nil {
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch wallet balance for %s: %v", req.CollateralAssetSymbol, err))
	}
	if userCollateralAvailableInWallet.LessThan(collateralAmountDecimal) {
		return NewError(http.StatusBadRequest, fmt.Sprintf("Insufficient common balance of %s for specified collateral. Available: %s, Required: %s",
			req.CollateralAssetSymbol, userCollateralAvailableInWallet.StringFixed(collateralToken.Decimals), collateralAmountDecimal.StringFixed(collateralToken.Decimals)))
	}

	// --- Fetch Current Margin Account State ---
	currentAccountDetails, err := sdk_wrappers.GetAccountDetails(hydroSDK, userAddress, marketFromDB.ID_uint16())
	if err != nil {
		utils.Warningf("OpenMarginPosition: Could not fetch current account details for user %s, market %s (may be new account): %v", userAddress.Hex(), marketFromDB.ID, err)
		currentAccountDetails = &sdk_wrappers.SDKAccountDetails{ 
			AssetsTotalUSDValue: big.NewInt(0),
			DebtsTotalUSDValue:  big.NewInt(0),
			Status:              0, 
			Liquidatable:        false,
		}
	}
	currentAssetsUSD_dec := utils.BigIntToDecimal(currentAccountDetails.AssetsTotalUSDValue, 18) 
	currentDebtsUSD_dec := utils.BigIntToDecimal(currentAccountDetails.DebtsTotalUSDValue, 18)

	// --- Calculations for Validation ---
	equityContributionDecimal := collateralAmountDecimal
	
	collateralAssetPriceUSD_dec, err := getPriceUSD(hydroSDK, collateralAssetAddress, collateralToken.Symbol)
	if err != nil { return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to get price for collateral asset %s: %v", req.CollateralAssetSymbol, err)) }
	if collateralAssetPriceUSD_dec.IsZero() {
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Oracle price for collateral asset %s is zero. Cannot perform USD calculations.", req.CollateralAssetSymbol))
	}
	equityContributionUSD_dec := equityContributionDecimal.Mul(collateralAssetPriceUSD_dec)

	// Simplified Borrow Amount Calculation (in USD terms) for validation
	borrowAmountUSD_dec := equityContributionUSD_dec.Mul(leverageDecimal.Sub(decimal.NewFromInt(1)))
	if borrowAmountUSD_dec.IsNegative() { borrowAmountUSD_dec = decimal.Zero }
	
	// --- Projected Account State for Validation ---
	projectedAssetsTotalUSD_dec := currentAssetsUSD_dec.Add(equityContributionUSD_dec).Add(borrowAmountUSD_dec)
	projectedDebtsTotalUSD_dec := currentDebtsUSD_dec.Add(borrowAmountUSD_dec)
	
	var projectedMarginRatio_dec decimal.Decimal
	if projectedDebtsTotalUSD_dec.IsPositive() {
		projectedMarginRatio_dec = projectedAssetsTotalUSD_dec.Div(projectedDebtsTotalUSD_dec)
	} else if projectedAssetsTotalUSD_dec.IsPositive() { 
		projectedMarginRatio_dec = decimal.NewFromInt(999999) 
	} else { 
		projectedMarginRatio_dec = decimal.NewFromInt(999999) 
	}
	utils.Infof("Pre-Trade Check for user %s, market %s: Requested Leverage: %s. Collateral Added (USD): %s. Borrowed (USD estimate): %s. Projected Assets USD: %s, Projected Debts USD: %s, Projected Margin Ratio: %s",
		userAddress.Hex(), marketFromDB.ID, req.Leverage, equityContributionUSD_dec.StringFixed(2), borrowAmountUSD_dec.StringFixed(2), projectedAssetsTotalUSD_dec.StringFixed(2), projectedDebtsTotalUSD_dec.StringFixed(2), projectedMarginRatio_dec.StringFixed(4))

	// --- Initial Margin Requirement (IMR) Check ---
	minInitialMarginRatio_dec := decimal.NewFromInt(1).Div(decimal.NewFromInt(1).Sub(imrFraction_dec))
	if projectedMarginRatio_dec.LessThan(minInitialMarginRatio_dec) {
		return NewError(http.StatusBadRequest, fmt.Sprintf("Projected margin ratio %s%% is below initial requirement %s%%. Reduce leverage or increase collateral.",
			projectedMarginRatio_dec.Mul(decimal.NewFromInt(100)).StringFixed(2), minInitialMarginRatio_dec.Mul(decimal.NewFromInt(100)).StringFixed(2)))
	}

	// --- Liquidation Rate Check ---
	if projectedMarginRatio_dec.LessThan(liquidateRate_dec) {
		return NewError(http.StatusBadRequest, fmt.Sprintf("Projected margin ratio %s%% is below liquidation rate %s%%. Trade would be instantly liquidated.",
			projectedMarginRatio_dec.Mul(decimal.NewFromInt(100)).StringFixed(2), liquidateRate_dec.Mul(decimal.NewFromInt(100)).StringFixed(2)))
	}

	utils.Info("Pre-trade validations passed for OpenMarginPosition.")
	// --- End of Pre-trade Validations ---

	// --- Prepare SDKBatchAction array ---
	var actions []sdk_wrappers.SDKBatchAction

	// Action 1: Transfer Collateral
	collateralAmountBigInt := utils.DecimalToBigInt(collateralAmountDecimal, int32(collateralToken.Decimals))
	fromPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCommon,
		MarketID: 0, 
		User:     userAddress,
	}
	toPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCollateralAccount,
		MarketID: marketFromDB.ID_uint16(), 
		User:     userAddress,
	}
	encodedTransferParams, err := sdk_wrappers.EncodeTransferParamsForBatch(collateralAssetAddress, fromPath, toPath, collateralAmountBigInt)
	if err != nil {
		utils.Errorf("Failed to encode transfer params for batch: %v", err)
		return NewError(http.StatusInternalServerError, "Failed to prepare collateral transfer action")
	}
	actions = append(actions, sdk_wrappers.SDKBatchAction{
		ActionType:    sdk_wrappers.SDKActionTypeTransfer,
		EncodedParams: encodedTransferParams,
	})
	utils.Info("Prepared Action 1: Transfer Collateral (%s %s) for market %s", req.CollateralAmount, req.CollateralAssetSymbol, marketFromDB.ID)

	// Action 2: Borrow Asset
	// TODO: This is where the precise borrow amount calculation is CRITICAL.
	// The `borrowAmountUSD_dec` used in validation is an estimate.
	// The actual amount to borrow in terms of base/quote asset needs to be calculated here.
	var borrowAssetAddress goEthereumCommon.Address
	var borrowAmountBigInt *big.Int
	var borrowedAssetToken *models.Token

	positionBaseAmountDecimal := amountDecimal
	positionQuoteValueDecimal := positionBaseAmountDecimal.Mul(priceDecimal)

	if req.Side == "buy" { 
		borrowAssetAddress = goEthereumCommon.HexToAddress(marketFromDB.QuoteTokenAddress)
		borrowedAssetToken, _ = models.TokenDao.GetTokenByAddress(marketFromDB.QuoteTokenAddress)
		if borrowedAssetToken == nil { return NewError(http.StatusInternalServerError, "Could not find quote token details for borrow.")}
		
		var equityInQuoteTokenDecimal decimal.Decimal
		if collateralAssetAddress == borrowAssetAddress { 
			equityInQuoteTokenDecimal = equityContributionDecimal
		} else { 
			// Collateral is Base, convert its value to Quote. Requires baseAssetPriceUSD.
			baseAssetPriceUSD_dec, err := getPriceUSD(hydroSDK, goEthereumCommon.HexToAddress(marketFromDB.BaseTokenAddress), marketFromDB.BaseTokenSymbol)
			if err != nil { return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to get price for base asset %s: %v", marketFromDB.BaseTokenSymbol, err)) }
			if priceDecimal.IsZero() {return NewError(http.StatusInternalServerError, "Base asset price cannot be zero for this calculation.")} // Should use quote price from oracle
            quoteAssetPriceUSD_dec, err := getPriceUSD(hydroSDK, goEthereumCommon.HexToAddress(marketFromDB.QuoteTokenAddress), marketFromDB.QuoteTokenSymbol)
            if err != nil { return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to get price for quote asset %s: %v", marketFromDB.QuoteTokenSymbol, err)) }
            if quoteAssetPriceUSD_dec.IsZero() { return NewError(http.StatusInternalServerError, "Quote asset price is zero, cannot convert collateral value.")}

			equityInQuoteTokenDecimal = equityContributionDecimal.Mul(baseAssetPriceUSD_dec).Div(quoteAssetPriceUSD_dec)
		}
		borrowAmountQuoteDecimal := positionQuoteValueDecimal.Sub(equityInQuoteTokenDecimal)
		if borrowAmountQuoteDecimal.LessThanOrEqual(decimal.Zero) { // If leverage is 1x or collateral covers full position
		    utils.Info("Calculated zero or negative borrow amount for quote asset. No borrow action needed for this asset.")
		    borrowAmountBigInt = big.NewInt(0)
		} else {
			borrowAmountBigInt = utils.DecimalToBigInt(borrowAmountQuoteDecimal, int32(borrowedAssetToken.Decimals))
			utils.Infof("Calculated Borrow: Quote Asset %s, Amount %s", borrowedAssetToken.Symbol, borrowAmountQuoteDecimal.StringFixed(borrowedAssetToken.Decimals))
		}
	} else { // Selling base asset, so borrowing base asset
		borrowAssetAddress = goEthereumCommon.HexToAddress(marketFromDB.BaseTokenAddress)
		borrowedAssetToken, _ = models.TokenDao.GetTokenByAddress(marketFromDB.BaseTokenAddress)
		if borrowedAssetToken == nil { return NewError(http.StatusInternalServerError, "Could not find base token details for borrow.")}

		var equityInBaseTokenDecimal decimal.Decimal
		if collateralAssetAddress == borrowAssetAddress { 
			equityInBaseTokenDecimal = equityContributionDecimal
		} else { // Collateral is Quote, convert its value to Base terms.
			if priceDecimal.IsZero() { return NewError(http.StatusBadRequest, "Price cannot be zero for this calculation.")}
			equityInBaseTokenDecimal = equityContributionDecimal.Div(priceDecimal) 
		}
		borrowAmountBaseDecimal := positionBaseAmountDecimal.Sub(equityInBaseTokenDecimal)
		if borrowAmountBaseDecimal.LessThanOrEqual(decimal.Zero) {
			utils.Info("Calculated zero or negative borrow amount for base asset. No borrow action needed for this asset.")
			borrowAmountBigInt = big.NewInt(0)
		} else {
			borrowAmountBigInt = utils.DecimalToBigInt(borrowAmountBaseDecimal, int32(borrowedAssetToken.Decimals))
			utils.Infof("Calculated Borrow: Base Asset %s, Amount %s", borrowedAssetToken.Symbol, borrowAmountBaseDecimal.StringFixed(borrowedAssetToken.Decimals))
		}
	}
	
	if borrowAmountBigInt != nil && borrowAmountBigInt.Sign() > 0 {
		encodedBorrowParams, err := sdk_wrappers.EncodeBorrowParamsForBatch(marketFromDB.ID_uint16(), borrowAssetAddress, borrowAmountBigInt)
		if err != nil {
			utils.Errorf("Failed to encode borrow params for batch: %v", err)
			return NewError(http.StatusInternalServerError, "Failed to prepare borrow action")
		}
		actions = append(actions, sdk_wrappers.SDKBatchAction{
			ActionType:    sdk_wrappers.SDKActionTypeBorrow,
			EncodedParams: encodedBorrowParams,
		})
		utils.Info("Prepared Action 2: Borrow Asset")
	} else {
		utils.Info("Skipping borrow action as calculated borrow amount is zero or less.")
	}

	if len(actions) == 0 {
		return NewError(http.StatusBadRequest, "No actions to perform. This might happen if collateral transfer is the only action and it's zero, or borrow amount is zero.")
	}

	// --- Get Unsigned Transaction ---
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
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to pack batch actions: %v", err))
	}

	// TODO: Get Nonce, GasPrice, GasLimit from appropriate sources
	nonce := uint64(0) 
	gasPrice := big.NewInt(20000000000) // Example: 20 Gwei
	gasLimit := uint64(1000000)    // Example: 1 Million gas limit for batch
	chainIdBigInt, _ := hydroSDK.GetChainID(context.Background())


	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     userAddress.Hex(),
		To:       sdk_wrappers.MarginContractAddress.Hex(),
		Value:    "0", 
		Data:     goEthereumCommon.Bytes2Hex(packedBatchData),
		Nonce:    fmt.Sprintf("%d", nonce),
		GasPrice: gasPrice.String(), 
		GasLimit: fmt.Sprintf("%d", gasLimit),
		ChainID:  chainIdBigInt.String(),
	}
	
	utils.Info("Successfully prepared unsigned transaction for opening margin position.")
	return c.JSON(http.StatusOK, unsignedTxForClient)
}

// CloseMarginPositionReq defines the request structure for closing a margin position.
// For now, it assumes full closure of all debts in the specified market.
type CloseMarginPositionReq struct {
	MarketID string `json:"marketID" validate:"required"` 
}

// CloseMarginPosition handles the request to close a margin position in a given market.
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
	marketUint16ID := market.ID_uint16() 

	hydroSDK := GetHydroSDK()
	if hydroSDK == nil {
		return NewError(http.StatusInternalServerError, "Hydro SDK not available")
	}

	var actions []sdk_wrappers.SDKBatchAction

	baseTokenAddr := goEthereumCommon.HexToAddress(market.BaseTokenAddress)
	quoteTokenAddr := goEthereumCommon.HexToAddress(market.QuoteTokenAddress)

	baseBorrowed, err := sdk_wrappers.GetAmountBorrowed(hydroSDK, userAddress, marketUint16ID, baseTokenAddr)
	if err != nil {
		utils.Errorf("Failed to get base amount borrowed for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		baseBorrowed = big.NewInt(0)
	} else {
		utils.Infof("User %s borrowed %s of base asset %s in market %s", userAddress.Hex(), baseBorrowed.String(), market.BaseTokenSymbol, req.MarketID)
	}

	quoteBorrowed, err := sdk_wrappers.GetAmountBorrowed(hydroSDK, userAddress, marketUint16ID, quoteTokenAddr)
	if err != nil {
		utils.Errorf("Failed to get quote amount borrowed for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		quoteBorrowed = big.NewInt(0)
	} else {
		utils.Infof("User %s borrowed %s of quote asset %s in market %s", userAddress.Hex(), quoteBorrowed.String(), market.QuoteTokenSymbol, req.MarketID)
	}

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

	baseBalanceInMarginAccount, err := sdk_wrappers.MarketBalanceOf(hydroSDK, marketUint16ID, baseTokenAddr, userAddress)
	if err != nil {
		utils.Errorf("Failed to get base balance in margin account for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		baseBalanceInMarginAccount = big.NewInt(0) 
	} else {
		utils.Infof("User %s has %s of base asset %s in margin account for market %s", userAddress.Hex(), baseBalanceInMarginAccount.String(), market.BaseTokenSymbol, req.MarketID)
	}

	quoteBalanceInMarginAccount, err := sdk_wrappers.MarketBalanceOf(hydroSDK, marketUint16ID, quoteTokenAddr, userAddress)
	if err != nil {
		utils.Errorf("Failed to get quote balance in margin account for market %s, user %s: %v", req.MarketID, userAddress.Hex(), err)
		quoteBalanceInMarginAccount = big.NewInt(0) 
	} else {
		utils.Infof("User %s has %s of quote asset %s in margin account for market %s", userAddress.Hex(), quoteBalanceInMarginAccount.String(), market.QuoteTokenSymbol, req.MarketID)
	}
	
	fromMarginPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCollateralAccount,
		MarketID: marketUint16ID,
		User:     userAddress,
	}
	toCommonPath := sdk_wrappers.SDKBalancePath{
		Category: sdk_wrappers.SDKBalanceCategoryCommon,
		MarketID: 0, 
		User:     userAddress,
	}

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

	nonce := uint64(0) 
	gasPrice := big.NewInt(20000000000) 
	gasLimit := uint64(1000000)    
	chainIdBigInt, _ := hydroSDK.GetChainID(context.Background())


	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     userAddress.Hex(),
		To:       sdk_wrappers.MarginContractAddress.Hex(),
		Value:    "0",
		Data:     goEthereumCommon.Bytes2Hex(packedBatchData),
		Nonce:    fmt.Sprintf("%d", nonce),
		GasPrice: gasPrice.String(), 
		GasLimit: fmt.Sprintf("%d", gasLimit),
		ChainID: chainIdBigInt.String(),
	}

	utils.Info("Successfully prepared unsigned transaction for closing margin position in market %s.", req.MarketID)
	return c.JSON(http.StatusOK, unsignedTxForClient)
}
