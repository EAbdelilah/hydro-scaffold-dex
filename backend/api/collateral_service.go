package api

import (
	"fmt"
	"net/http"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	sw "github.com/HydroProtocol/hydro-scaffold-dex/backend/sdk_wrappers" // SDK Wrappers
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"strings"
	// "github.com/HydroProtocol/hydro-sdk-go/sdk" // Placeholder for actual SDK package
)

// GetMarginAccountDetails handles the request to fetch margin account details for a given market and user.
func GetMarginAccountDetails(p Param) (interface{}, error) {
	req := p.(*MarginAccountDetailsReq)

	// TODO: Get userAddress from authenticated context c.Get("userID").(string)
	// For now, using req.UserAddress which is validated to be an eth_addr
	userAddress := req.UserAddress

	market := models.MarketDao.FindMarketByID(req.MarketID)
	if market == nil {
		return nil, NewApiError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	if !market.BorrowEnable {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Margin trading not enabled for market %s", req.MarketID))
	}

	resp := MarginAccountDetailsResp{
		MarketID:    req.MarketID,
		UserAddress: userAddress,
	}

	// Placeholder for SDK's AccountDetails structure
	// type AccountDetailsResultStruct struct {
	// 	Status              int       // e.g. 0 for Normal, 1 for MarginCall, 2 for Liquidated
	// 	CollateralValueUSD  *big.Int
	// 	DebtValueUSD        *big.Int
	//  Liquidatable        bool
	// }

	commonUserAddress := common.HexToAddress(userAddress)
	uint16MarketID, err := sw.MarketIDToUint16(req.MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID format: %v", err))
	}

	accountDetails, err := sw.GetAccountDetails(hydro, commonUserAddress, uint16MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch account details: %v", err))
	}

	resp.Liquidatable = accountDetails.Liquidatable
	// Assuming USD values from contract have 18 decimals
	resp.AssetsTotalUSDValue = utils.WeiToDecimalAmount(accountDetails.AssetsTotalUSDValue, 18)
	resp.DebtsTotalUSDValue = utils.WeiToDecimalAmount(accountDetails.DebtsTotalUSDValue, 18)

	switch accountDetails.Status {
	case 0:
		resp.Status = "Normal"
	case 1:
		resp.Status = "MarginCall"
	case 2:
		resp.Status = "Liquidated"
	default:
		resp.Status = "Unknown"
	}

	if resp.DebtsTotalUSDValue.GreaterThan(decimal.Zero) {
		resp.CollateralRatio = resp.AssetsTotalUSDValue.Div(resp.DebtsTotalUSDValue)
	} else {
		resp.CollateralRatio = decimal.Zero // Or omit if preferred for zero debt
	}

	// --- Base Asset Details ---
	// --- Base Asset Details ---
	baseTotalBalanceBigInt, err := sw.MarketBalanceOf(hydro, uint16MarketID, common.HexToAddress(market.BaseTokenAddress), commonUserAddress)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch base token collateral balance: %v", err))
	}
	baseTransferableBigInt, err := sw.GetMarketTransferableAmount(hydro, uint16MarketID, common.HexToAddress(market.BaseTokenAddress), commonUserAddress)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch base token transferable amount: %v", err))
	}

	resp.BaseAssetDetails = MarginAssetDetails{
		AssetAddress:       market.BaseTokenAddress,
		Symbol:             market.BaseTokenSymbol,
		TotalBalance:       utils.WeiToDecimalAmount(baseTotalBalanceBigInt, market.BaseTokenDecimals),
		TransferableAmount: utils.WeiToDecimalAmount(baseTransferableBigInt, market.BaseTokenDecimals),
	}

	// --- Quote Asset Details ---
	quoteTotalBalanceBigInt, err := sw.MarketBalanceOf(hydro, uint16MarketID, common.HexToAddress(market.QuoteTokenAddress), commonUserAddress)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch quote token collateral balance: %v", err))
	}
	quoteTransferableBigInt, err := sw.GetMarketTransferableAmount(hydro, uint16MarketID, common.HexToAddress(market.QuoteTokenAddress), commonUserAddress)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch quote token transferable amount: %v", err))
	}

	resp.QuoteAssetDetails = MarginAssetDetails{
		AssetAddress:       market.QuoteTokenAddress,
		Symbol:             market.QuoteTokenSymbol,
		TotalBalance:       utils.WeiToDecimalAmount(quoteTotalBalanceBigInt, market.QuoteTokenDecimals),
		TransferableAmount: utils.WeiToDecimalAmount(quoteTransferableBigInt, market.QuoteTokenDecimals),
	}

	return resp, nil
}

// DepositToCollateral handles depositing assets into a user's margin account for a specific market.
func DepositToCollateral(p Param) (interface{}, error) {
	req := p.(*CollateralManagementReq)

	// TODO: Get userAddress from authenticated context c.Get("userID").(string)
	// For now, using req.UserAddress from BaseReq which should be set by auth middleware if not provided in body.
	// If auth middleware is not yet setting BaseReq.Address, this will be empty.
	// A more robust approach would be to ensure auth sets it, or explicitly pass c echo.Context here.
	reqUserAddress := req.GetAddress() // This is string from BaseReq
	if reqUserAddress == "" {
		// This should ideally be caught by auth middleware or validation on BaseReq
		reqUserAddress = "0xSIMULATEDUSERADDRESSFORPOST" // Fallback placeholder
	}
	commonUserAddress := common.HexToAddress(reqUserAddress)

	if !req.Amount.IsPositive() {
		return nil, NewApiError(http.StatusBadRequest, "Amount must be positive")
	}

	market := models.MarketDao.FindMarketByID(req.MarketID)
	if market == nil {
		return nil, NewApiError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	if !market.BorrowEnable {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Margin trading not enabled for market %s", req.MarketID))
	}

	if req.AssetAddress != market.BaseTokenAddress && req.AssetAddress != market.QuoteTokenAddress {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid asset address %s for market %s. Must be base or quote token.", req.AssetAddress, req.MarketID))
	}

	var tokenDecimals int
	if req.AssetAddress == market.BaseTokenAddress {
		tokenDecimals = market.BaseTokenDecimals
	} else {
		tokenDecimals = market.QuoteTokenDecimals
	}

	// Check common balance (using existing hydro SDK direct call for now, not a wrapper)
	// This assumes hydro.GetTokenBalance returns balance in normal units (needs conversion to compare with req.Amount)
	commonBalanceBigInt := hydro.GetTokenBalance(req.AssetAddress, reqUserAddress) // req.AssetAddress is string, hydro.GetTokenBalance might need common.Address
	commonBalance := utils.WeiToDecimalAmount(commonBalanceBigInt, tokenDecimals)
	if req.Amount.GreaterThan(commonBalance) {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Insufficient common balance for %s. Have: %s, Need: %s",
			market.GetTokenSymbolByAddress(req.AssetAddress), commonBalance.String(), req.Amount.String()))
	}
	utils.Dump("Common balance check passed for", reqUserAddress, req.AssetAddress, "amount", req.Amount.String())

	uint16MarketID, err := sw.MarketIDToUint16(req.MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID format for SDK: %v", err))
	}

	fromPath := sw.SDKBalancePath{
		Category: sw.SDKBalanceCategoryCommon,
		MarketID: 0, // Common balance is not market-specific in this context
		User:     commonUserAddress,
	}
	toPath := sw.SDKBalancePath{
		Category: sw.SDKBalanceCategoryCollateralAccount,
		MarketID: uint16MarketID,
		User:     commonUserAddress,
	}
	amountBigInt := utils.DecimalToWei(req.Amount, tokenDecimals)

	encodedParams, err := sw.EncodeTransferParamsForBatch(common.HexToAddress(req.AssetAddress), fromPath, toPath, amountBigInt)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to encode transfer params: %v", err))
	}

	action := sw.SDKBatchAction{
		ActionType:    sw.SDKActionTypeTransfer, // Assuming Transfer is the correct type for deposit from common to collateral
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
		return nil, NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to pack batch actions for deposit: %v", err))
	}

	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     commonUserAddress.Hex(),
		To:       sw.MarginContractAddress.Hex(),
		Value:    "0", // Assuming no ETH value sent directly to batch function
		Data:     common.Bytes2Hex(packedBatchData),
		GasPrice: "0", // Placeholder - should be estimated or from config
		GasLimit: "0", // Placeholder - should be estimated
	}
	utils.Info("Prepared unsigned transaction for collateral deposit.")
	return unsignedTxForClient, nil
}

// WithdrawFromCollateral handles withdrawing assets from a user's margin account for a specific market.
func WithdrawFromCollateral(p Param) (interface{}, error) {
	req := p.(*CollateralManagementReq)

	// TODO: Get userAddress from authenticated context c.Get("userID").(string)
	reqUserAddress := req.GetAddress()
	if reqUserAddress == "" {
		reqUserAddress = "0xSIMULATEDUSERADDRESSFORPOST" // Fallback placeholder
	}
	commonUserAddress := common.HexToAddress(reqUserAddress)

	if !req.Amount.IsPositive() {
		return nil, NewApiError(http.StatusBadRequest, "Amount must be positive")
	}

	market := models.MarketDao.FindMarketByID(req.MarketID)
	if market == nil {
		return nil, NewApiError(http.StatusNotFound, fmt.Sprintf("Market %s not found", req.MarketID))
	}
	if !market.BorrowEnable {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Margin trading not enabled for market %s", req.MarketID))
	}

	if req.AssetAddress != market.BaseTokenAddress && req.AssetAddress != market.QuoteTokenAddress {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid asset address %s for market %s. Must be base or quote token.", req.AssetAddress, req.MarketID))
	}

	var tokenDecimals int
	if req.AssetAddress == market.BaseTokenAddress {
		tokenDecimals = market.BaseTokenDecimals
	} else {
		tokenDecimals = market.QuoteTokenDecimals
	}

	uint16MarketID, err := sw.MarketIDToUint16(req.MarketID)
	if err != nil {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID format for SDK: %v", err))
	}
	commonAssetAddress := common.HexToAddress(req.AssetAddress)

	transferableAmountBigInt, err := sw.GetMarketTransferableAmount(hydro, uint16MarketID, commonAssetAddress, commonUserAddress)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to get transferable amount: %v", err))
	}
	transferableAmount := utils.WeiToDecimalAmount(transferableAmountBigInt, tokenDecimals)

	if req.Amount.GreaterThan(transferableAmount) {
		return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Withdraw amount %s exceeds transferable amount %s for %s",
			req.Amount.String(), transferableAmount.String(), market.GetTokenSymbolByAddress(req.AssetAddress)))
	}

	fromPath := sw.SDKBalancePath{
		Category: sw.SDKBalanceCategoryCollateralAccount,
		MarketID: uint16MarketID,
		User:     commonUserAddress,
	}
	toPath := sw.SDKBalancePath{
		Category: sw.SDKBalanceCategoryCommon,
		MarketID: 0, // Common balance is not market-specific
		User:     commonUserAddress,
	}
	amountBigInt := utils.DecimalToWei(req.Amount, tokenDecimals)

	encodedParams, err := sw.EncodeTransferParamsForBatch(commonAssetAddress, fromPath, toPath, amountBigInt)
	if err != nil {
		return nil, NewApiError(http.StatusInternalServerError, fmt.Sprintf("Failed to encode transfer params for withdraw: %v", err))
	}

	action := sw.SDKBatchAction{
		ActionType:    sw.SDKActionTypeTransfer, // Assuming Transfer is also used for withdrawing from collateral to common
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
		return nil, NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to pack batch actions for withdraw: %v", err))
	}

	unsignedTxForClient := &common.UnsignedTxDataForClient{
		From:     commonUserAddress.Hex(),
		To:       sw.MarginContractAddress.Hex(),
		Value:    "0", // Assuming no ETH value sent directly to batch function
		Data:     common.Bytes2Hex(packedBatchData),
		GasPrice: "0", // Placeholder - should be estimated or from config
		GasLimit: "0", // Placeholder - should be estimated
	}
	utils.Info("Prepared unsigned transaction for collateral withdrawal.")
	return unsignedTxForClient, nil
}
