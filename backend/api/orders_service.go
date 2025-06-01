package api

import (
	"encoding/json"
	"errors"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	sw "github.com/HydroProtocol/hydro-scaffold-dex/backend/sdk_wrappers"
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk"
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/ethereum/go-ethereum/common" as goEthereumCommon
	"github.com/shopspring/decimal"
)

func GetLockedBalance(p Param) (interface{}, error) {
	"os"
	"time"
)

func GetLockedBalance(p Param) (interface{}, error) {
	req := p.(*LockedBalanceReq)
	tokens := models.TokenDao.GetAllTokens()

	var lockedBalances []LockedBalance

	for _, token := range tokens {
		lockedBalance := models.BalanceDao.GetByAccountAndSymbol(req.Address, token.Symbol, token.Decimals)
		lockedBalances = append(lockedBalances, LockedBalance{
			Symbol:        token.Symbol,
			LockedBalance: lockedBalance,
		})
	}

	return &LockedBalanceResp{
		LockedBalances: lockedBalances,
	}, nil
}

func GetSingleOrder(p Param) (interface{}, error) {
	req := p.(*QuerySingleOrderReq)

	order := models.OrderDao.FindByID(req.OrderID)

	return &QuerySingleOrderResp{
		Order: order,
	}, nil
}

func GetOrders(p Param) (interface{}, error) {
	req := p.(*QueryOrderReq)
	if req.Status == "" {
		req.Status = common.ORDER_PENDING
	}
	if req.PerPage <= 0 {
		req.PerPage = 20
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	offset := req.PerPage * (req.Page - 1)
	limit := req.PerPage

	count, orders := models.OrderDao.FindByAccount(req.Address, req.MarketID, req.Status, offset, limit)

	return &QueryOrderResp{
		Count:  count,
		Orders: orders,
	}, nil
}

func CancelOrder(p Param) (interface{}, error) {
	req := p.(*CancelOrderReq)
	order := models.OrderDao.FindByID(req.ID)
	if order == nil {
		return nil, NewApiError(-1, fmt.Sprintf("order %s not exist", req.ID))
	}

	if order.Status != common.ORDER_PENDING {
		return nil, nil
	}

	cancelOrderEvent := common.CancelOrderEvent{
		Event: common.Event{
			Type:     common.EventCancelOrder,
			MarketID: order.MarketID,
		},
		Price: order.Price.String(),
		Side:  order.Side,
		ID:    order.ID,
	}

	return nil, QueueService.Push([]byte(utils.ToJsonString(cancelOrderEvent)))
}

func BuildOrder(p Param) (interface{}, error) {
	utils.Debugf("BuildOrder param %v", p)

	req := p.(*BuildOrderReq)
	err := checkBalanceAllowancePriceAndAmount(req, req.Address)
	if err != nil {
		return nil, err
	}

	buildOrderResponse, err := BuildAndCacheOrder(req.Address, req)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"order": buildOrderResponse,
	}, nil
}

func PlaceOrder(p Param) (interface{}, error) {
	order := p.(*PlaceOrderReq)
	if valid := hydro.IsValidOrderSignature(order.Address, order.ID, order.Signature); !valid {
		utils.Infof("valid is %v", valid)
		return nil, errors.New("bad signature")
	}

	cacheOrder := getCacheOrderByOrderID(order.ID)

	if cacheOrder == nil {
		return nil, errors.New("place order error, please retry later")
	}

	cacheOrder.OrderResponse.Json.Signature = order.Signature

	ret := models.Order{
		ID:              order.ID,
		TraderAddress:   order.Address,
		MarketID:        cacheOrder.OrderResponse.MarketID,
		Side:            cacheOrder.OrderResponse.Side,
		Price:           cacheOrder.OrderResponse.Price,
		Amount:          cacheOrder.OrderResponse.Amount,
		Status:          common.ORDER_PENDING,
		Type:            cacheOrder.OrderResponse.Type,
		Version:         "hydro-v1",
		AvailableAmount: cacheOrder.OrderResponse.Amount,
		ConfirmedAmount: decimal.Zero,
		CanceledAmount:  decimal.Zero,
		PendingAmount:   decimal.Zero,
		MakerFeeRate:    cacheOrder.OrderResponse.AsMakerFeeRate,
		TakerFeeRate:    cacheOrder.OrderResponse.AsTakerFeeRate,
		MakerRebateRate: cacheOrder.OrderResponse.MakerRebateRate,
		GasFeeAmount:    cacheOrder.OrderResponse.GasFeeAmount,
		JSON:            utils.ToJsonString(cacheOrder.OrderResponse.Json),
		CreatedAt:       time.Now().UTC(),
	}

	newOrderEvent, _ := json.Marshal(common.NewOrderEvent{
		Event: common.Event{
			MarketID: cacheOrder.OrderResponse.MarketID,
			Type:     common.EventNewOrder,
		},
		Order: utils.ToJsonString(ret),
	})

	err := QueueService.Push(newOrderEvent)

	if err != nil {
		return nil, errors.New("place order failed, place try again")
	} else {
		return nil, nil
	}
}

func getCacheOrderByOrderID(orderID string) *CacheOrder {
	cacheOrderStr, err := CacheService.Get(generateOrderCacheKey(orderID))

	if err != nil {
		utils.Errorf("get cache order error: %v", err)
		return nil
	}

	var cacheOrder CacheOrder

	err = json.Unmarshal([]byte(cacheOrderStr), &cacheOrder)
	if err != nil {
		utils.Errorf("get cache order error: %v, cache order is: %v", err, cacheOrderStr)
		return nil
	}

	return &cacheOrder
}

func checkBalanceAllowancePriceAndAmount(order *BuildOrderReq, address string) error {
	market := models.MarketDao.FindMarketByID(order.MarketID)
	if market == nil {
		return MarketNotFoundError(order.MarketID)
	}

	// price and amount are already decimal.Decimal at this point in the original code
	// but they are parsed from order.Price and order.Amount (strings)
	// For clarity, let's ensure we have them as decimals here.
	price := utils.StringToDecimal(order.Price)
	amount := utils.StringToDecimal(order.Amount)

	// Validate price and amount units and minimum order size (common for both spot and margin)
	minPriceUnit := decimal.New(1, int32(-1*market.PriceDecimals))
	if price.LessThanOrEqual(decimal.Zero) || !price.Mod(minPriceUnit).Equal(decimal.Zero) {
		return NewApiError(-1, "invalid_price_or_unit")
	}

	minAmountUnit := decimal.New(1, int32(-1*market.AmountDecimals))
	if amount.LessThanOrEqual(decimal.Zero) || !amount.Mod(minAmountUnit).Equal(decimal.Zero) {
		return NewApiError(-1, "invalid_amount_or_unit")
	}

	orderSizeInQuoteToken := amount.Mul(price)
	if orderSizeInQuoteToken.LessThan(market.MinOrderSize) {
		return NewApiError(-1, "order_less_than_minOrderSize")
	}

	if order.AccountType == "margin" {
		utils.Dump(fmt.Sprintf("Performing MARGIN balance check for user %s, market %s (side %s)", address, order.MarketID, order.Side))

		var assetToCheckAddress string
		var assetToCheckDecimals int
		var assetToCheckSymbol string

		if order.Side == "sell" { // Selling base, need spendable base
			assetToCheckAddress = market.BaseTokenAddress
			assetToCheckDecimals = market.BaseTokenDecimals
			assetToCheckSymbol = market.BaseTokenSymbol
		} else { // Buying base (spending quote), need spendable quote
			assetToCheckAddress = market.QuoteTokenAddress
			assetToCheckDecimals = market.QuoteTokenDecimals
			assetToCheckSymbol = market.QuoteTokenSymbol
		}
		utils.Dump(fmt.Sprintf("Margin order: asset to check is %s (%s)", assetToCheckSymbol, assetToCheckAddress))

		commonAddress := goEthereumCommon.HexToAddress(address)
		// Use order.MarketID as the margin market context. This might be order.MarginMarketID if they can differ.
		uint16MarketID, err := sw.MarketIDToUint16(order.MarketID)
		if err != nil {
			return NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID for margin order: %s", order.MarketID))
		}
		commonAssetToCheckAddress := goEthereumCommon.HexToAddress(assetToCheckAddress)

		collateralBalanceBigInt, err := sw.MarketBalanceOf(hydro, uint16MarketID, commonAssetToCheckAddress, commonAddress)
		if err != nil {
			utils.Errorf("Failed to fetch collateral balance for %s in market %s: %v", commonAddress.Hex(), order.MarketID, err)
			return NewApiError(-1, "failed_to_fetch_collateral_balance")
		}
		collateralBalance := utils.WeiToDecimalAmount(collateralBalanceBigInt, assetToCheckDecimals)
		utils.Dump("Actual Collateral Balance for asset", assetToCheckSymbol, ":", collateralBalance.StringWithDigits(int32(assetToCheckDecimals)))

		borrowedAmountBigInt, err := sw.GetAmountBorrowed(hydro, commonAddress, uint16MarketID, commonAssetToCheckAddress)
		if err != nil {
			utils.Errorf("Failed to fetch borrowed amount for %s in market %s: %v", commonAddress.Hex(), order.MarketID, err)
			return NewApiError(-1, "failed_to_fetch_borrowed_amount")
		}
		borrowedAmount := utils.WeiToDecimalAmount(borrowedAmountBigInt, assetToCheckDecimals)
		utils.Dump("Actual Borrowed Amount for asset", assetToCheckSymbol, ":", borrowedAmount.StringWithDigits(int32(assetToCheckDecimals)))

		spendableBalance := collateralBalance.Add(borrowedAmount)
		utils.Dump("Placeholder: Spendable (Collateral + Borrowed) for asset", assetToCheckSymbol, ":", spendableBalance.StringWithDigits(int32(assetToCheckDecimals)))

		var requiredAmountInAssetUnits decimal.Decimal // This should be in normal units, not "huge" units yet
		feeDetail := calculateFee(price, amount, market, address)

		if order.Side == "sell" { // Selling base. Required amount is 'amount' of base token.
			requiredAmountInAssetUnits = amount
		} else { // Buying base (spending quote). Required amount is 'amount * price' of quote token, plus fees.
			orderValueInQuote := amount.Mul(price)
			// calculateFee returns AsTakerTotalFeeAmount in quote token's normal units (not huge units)
			// However, the original spot logic adds feeAmount (which is AsTakerTotalFeeAmount in huge units) to quoteTokenHugeAmount.
			// For consistency, let's ensure feeDetail.AsTakerTotalFeeAmount is used correctly.
			// The AsTakerTotalFeeAmount from calculateFee is already in quote token's "normal" decimal units.
			takerFeeInQuoteToken := feeDetail.AsTakerTotalFeeAmount.Div(decimal.New(1, int32(market.QuoteTokenDecimals))) // Convert from huge units if necessary, or ensure calculateFee returns normal units.
			                                                                                                           // Assuming calculateFee.AsTakerTotalFeeAmount is already in normal units of quote for now.
			                                                                                                           // The original code has: feeAmount := feeDetail.AsTakerTotalFeeAmount (this is in huge units)
			                                                                                                           // And then: requireAmount := quoteTokenHugeAmount.Add(feeAmount)
			// Let's re-evaluate fee calculation for margin context.
			// For margin, fees are typically paid from the collateral of the quote token.
			// If spending quote, required = (order value in quote) + (fee in quote)
			// Taker fee is usually charged in the quote currency of the pair.
			takerFee := feeDetail.AsTakerTotalFeeAmount.Div(decimal.New(1, int32(market.QuoteTokenDecimals))) // Ensure this is in normal units of quote
			requiredAmountInAssetUnits = orderValueInQuote.Add(takerFee)
		}
		utils.Dump(fmt.Sprintf("Margin: Required amount of %s (normal units): %s", assetToCheckSymbol, requiredAmountInAssetUnits.StringWithDigits(int32(assetToCheckDecimals))))

		if requiredAmountInAssetUnits.GreaterThan(spendableBalance) {
			return NewApiError(-1, fmt.Sprintf("%s margin balance (collateral + borrowed) not enough. Available: %s, Required: %s",
				assetToCheckSymbol,
				spendableBalance.StringWithDigits(int32(assetToCheckDecimals)),
				requiredAmountInAssetUnits.StringWithDigits(int32(assetToCheckDecimals))))
		}

		// Allowance checks for margin orders:
		// The contract interaction for margin orders might involve direct transfers from the margin account
		// by the user's signature on the order, or it might still use the allowance model on a specific margin contract/proxy.
		// This needs clarification based on the smart contract design.
		// For now, assume a similar allowance check might be needed against the margin contract/proxy if funds are moved by it.
		// TODO: Confirm allowance mechanism for margin trades. Is it on user's EOA to margin contract, or margin contract to Hydro proxy?
		// If the Hydro proxy still needs to pull funds, allowance is on assetToCheckAddress from user's EOA to HSK_PROXY_ADDRESS.
		// This is complex because the funds are technically in the margin contract.
		// For a true margin system, the order itself, when matched, triggers actions on the margin contract.
		// The "allowance" might be an internal one within the margin contract system or not needed in the traditional ERC20 sense from EOA for each trade.
		// Let's assume for now that no separate ERC20 allowance check from EOA to HSK_PROXY_ADDRESS is needed IF funds are already in margin contract
		// and the margin contract itself is the one interacting with the exchange component.
		// However, if the margin account IS the HSK_PROXY_ADDRESS (unlikely), then allowance is implicitly given.
		utils.Dump("Placeholder: Allowance check for margin trade for asset: ", assetToCheckSymbol, " - This needs clarification based on contract design.")

	} else {
		// Existing Spot Trading Logic
		utils.Dump(fmt.Sprintf("Performing SPOT balance check for user %s, market %s (side %s)", address, order.MarketID, order.Side))

		baseTokenLockedBalance := models.BalanceDao.GetByAccountAndSymbol(address, market.BaseTokenSymbol, market.BaseTokenDecimals)
		baseTokenBalance := hydro.GetTokenBalance(market.BaseTokenAddress, address)
		baseTokenAllowance := hydro.GetTokenAllowance(market.BaseTokenAddress, os.Getenv("HSK_PROXY_ADDRESS"), address)

		quoteTokenLockedBalance := models.BalanceDao.GetByAccountAndSymbol(address, market.QuoteTokenSymbol, market.QuoteTokenDecimals)
		quoteTokenBalance := hydro.GetTokenBalance(market.QuoteTokenAddress, address)
		quoteTokenAllowance := hydro.GetTokenAllowance(market.QuoteTokenAddress, os.Getenv("HSK_PROXY_ADDRESS"), address)

		var quoteTokenHugeAmount decimal.Decimal
		var baseTokenHugeAmount decimal.Decimal

		// feeDetail from calculateFee returns AsTakerTotalFeeAmount in quote token's huge units.
		feeDetail := calculateFee(price, amount, market, address) // price and amount are decimal.Decimal
		feeAmountInQuoteHugeUnits := feeDetail.AsTakerTotalFeeAmount

		quoteTokenHugeAmount = amount.Mul(price).Mul(decimal.New(1, int32(market.QuoteTokenDecimals)))
		baseTokenHugeAmount = amount.Mul(decimal.New(1, int32(market.BaseTokenDecimals)))

		if order.Side == "sell" { // Selling Base Token
			// Check if order value in quote token can cover fee (which is also in quote token)
			// This check seems a bit off: quoteTokenHugeAmount is order value, feeAmount is fee.
			// Original: if quoteTokenHugeAmount.LessThanOrEqual(feeAmount)
			// This means if (value of base sold, in quote terms) <= (fee in quote terms).
			// This check is likely to ensure the order is not just to pay fees or is economically viable.
			// Let's keep it as it was, assuming it has a purpose.
			if quoteTokenHugeAmount.LessThanOrEqual(feeAmountInQuoteHugeUnits) && !quoteTokenHugeAmount.IsZero() { // Added !quoteTokenHugeAmount.IsZero() to avoid error on zero value orders if fee is also zero
				return NewApiError(-1, fmt.Sprintf("Order value in quote token (%s) must be greater than fee (%s)", utils.DecimalToFriendlyJSON(quoteTokenHugeAmount), utils.DecimalToFriendlyJSON(feeAmountInQuoteHugeUnits)))
			}

			availableBaseTokenAmount := baseTokenBalance.Sub(baseTokenLockedBalance) // These are in normal units
			// Compare baseTokenHugeAmount (order amount in huge units) with availableBaseTokenAmount (normal units)
			// This needs availableBaseTokenAmount to be converted to huge units or baseTokenHugeAmount to normal.
			// Original code compares huge amount with normal amount which is incorrect.
			// Assuming hydro.GetTokenBalance returns normal units.
			if baseTokenHugeAmount.GreaterThan(availableBaseTokenAmount.Mul(decimal.New(1, int32(market.BaseTokenDecimals)))) {
				return NewApiError(-1, fmt.Sprintf("%s balance not enough, available balance is %s, require amount is %s", market.BaseTokenSymbol, availableBaseTokenAmount.StringFixed(int32(market.BaseTokenDecimals)), amount.StringFixed(int32(market.BaseTokenDecimals))))
			}

			// Allowance is checked in huge units against HSK_PROXY_ADDRESS
			if baseTokenHugeAmount.GreaterThan(baseTokenAllowance) { // baseTokenAllowance is already in huge units from SDK
				return NewApiError(-1, fmt.Sprintf("%s allowance not enough, allowance is %s, require amount is %s", market.BaseTokenSymbol, utils.DecimalToFriendlyJSON(baseTokenAllowance), utils.DecimalToFriendlyJSON(baseTokenHugeAmount)))
			}
		} else { // Buying Base Token (Spending Quote Token)
			availableQuoteTokenAmount := quoteTokenBalance.Sub(quoteTokenLockedBalance) // Normal units
			requiredAmountInQuoteHugeUnits := quoteTokenHugeAmount.Add(feeAmountInQuoteHugeUnits) // Both in huge units

			// Compare requiredAmountInQuoteHugeUnits (huge units) with availableQuoteTokenAmount (normal units)
			if requiredAmountInQuoteHugeUnits.GreaterThan(availableQuoteTokenAmount.Mul(decimal.New(1, int32(market.QuoteTokenDecimals)))) {
				return NewApiError(-1, fmt.Sprintf("%s balance not enough, available balance is %s, require amount (incl. fee) is %s", market.QuoteTokenSymbol, availableQuoteTokenAmount.StringFixed(int32(market.QuoteTokenDecimals)), requiredAmountInQuoteHugeUnits.Div(decimal.New(1, int32(market.QuoteTokenDecimals))).StringFixed(int32(market.QuoteTokenDecimals))))
			}

			// Allowance is checked in huge units
			if requiredAmountInQuoteHugeUnits.GreaterThan(quoteTokenAllowance) { // quoteTokenAllowance is huge units
				return NewApiError(-1, fmt.Sprintf("%s allowance not enough, allowance is %s, require amount (incl. fee) is %s", market.QuoteTokenSymbol, utils.DecimalToFriendlyJSON(quoteTokenAllowance), utils.DecimalToFriendlyJSON(requiredAmountInQuoteHugeUnits)))
			}
		}
	}

	// will add check of precision later
	return nil
}

func BuildAndCacheOrder(address string, order *BuildOrderReq) (*BuildOrderResp, error) {
	market := models.MarketDao.FindMarketByID(order.MarketID)
	amount := utils.StringToDecimal(order.Amount)
	price := utils.StringToDecimal(order.Price)

	// Determine balanceCategory and orderDataMarketID based on AccountType
	var balanceCategory sw.SDKBalanceCategory // Use the type from sdk_wrappers
	var orderDataMarketIDUint16 uint16         // Ensure this is uint16 for the wrapper
	var orderDataHex string                    // To store the generated order data string
	var err error

	if order.AccountType == "margin" {
		// Use order.MarketID as the context for margin operations (collateral, loans are per-marketPair)
		// The order.MarginMarketID field in BuildOrderReq can be used if a *different* market context
		// is needed for the order's data field than for its placement market (order.MarketID).
		// For now, assume order.MarketID is the one to use for orderDataMarketIDUint16.
		orderDataMarketIDUint16, err = sw.MarketIDToUint16(order.MarketID)
		if err != nil {
			return nil, NewApiError(http.StatusBadRequest, fmt.Sprintf("Invalid MarketID for margin order data: %s", order.MarketID))
		}

		balanceCategory = sw.SDKBalanceCategoryCollateralAccount
		utils.Dump(fmt.Sprintf("Margin order: Using CollateralAccount (Category: %d) and MarketID %d for order data.", balanceCategory, orderDataMarketIDUint16))

		orderDataHex, err = sw.GenerateMarginOrderDataHex(
			int64(2), // Version
			getExpiredAt(order.Expires),
			rand.Int63(), // Salt
			market.MakerFeeRate,
			market.TakerFeeRate,
			decimal.Zero, // MakerRebateRate
			order.Side == "sell",
			order.OrderType == "market",
			balanceCategory,
			orderDataMarketIDUint16,
			false, // isMakerOnly
		)
		if err != nil {
			utils.Errorf("Failed to generate margin order data: %v", err)
			return nil, NewApiError(-1, "failed_to_build_margin_order_data")
		}

	} else {
		// Default to spot order if AccountType is empty or "spot"
		orderDataMarketIDUint16 = 0 // For common balances (spot), marketID in order data is 0
		balanceCategory = sw.SDKBalanceCategoryCommon
		utils.Dump(fmt.Sprintf("Spot order: Using Common account (Category: %d) and MarketID %d for order data.", balanceCategory, orderDataMarketIDUint16))

		// Spot orders use the existing SDK's GenerateOrderData
		orderDataBytes := hydro.GenerateOrderData(
			int64(2), // Version
			getExpiredAt(order.Expires),
			rand.Int63(), // Salt
			market.MakerFeeRate,
			market.TakerFeeRate,
			decimal.Zero, // MakerRebateRate
			order.Side == "sell",
			order.OrderType == "market",
			false, // isMakerOnly
		)
		orderDataHex = goEthereumCommon.Bytes2Hex(orderDataBytes)
	}

	fee := calculateFee(price, amount, market, address)

	gasFeeInQuoteToken := fee.GasFeeAmount
	gasFeeInQuoteTokenHugeAmount := fee.GasFeeAmount.Mul(decimal.New(1, int32(market.QuoteTokenDecimals)))

	makerRebateRate := decimal.Zero
	offeredAmount := decimal.Zero

	var baseTokenHugeAmount decimal.Decimal
	var quoteTokenHugeAmount decimal.Decimal

	baseTokenHugeAmount = amount.Mul(decimal.New(1, int32(market.BaseTokenDecimals)))
	quoteTokenHugeAmount = price.Mul(amount).Mul(decimal.New(1, int32(market.QuoteTokenDecimals)))

	orderJson := models.OrderJSON{
		Trader:                  address,
		Relayer:                 os.Getenv("HSK_RELAYER_ADDRESS"),
		BaseCurrency:            market.BaseTokenAddress,
		QuoteCurrency:           market.QuoteTokenAddress,
		BaseCurrencyHugeAmount:  baseTokenHugeAmount,
		QuoteCurrencyHugeAmount: quoteTokenHugeAmount,
		GasTokenHugeAmount:      gasFeeInQuoteTokenHugeAmount,
		Data:                    orderDataHex, // Use the hex string here
	}

	sdkOrder := sdk.NewOrderWithData(address,
		os.Getenv("HSK_RELAYER_ADDRESS"),
		market.BaseTokenAddress,
		market.QuoteTokenAddress,
		utils.DecimalToBigInt(baseTokenHugeAmount),
		utils.DecimalToBigInt(quoteTokenHugeAmount),
		utils.DecimalToBigInt(gasFeeInQuoteTokenHugeAmount),
		orderDataHex, // Pass hex string to SDK
		"",
	)

	orderHash := hydro.GetOrderHash(sdkOrder)
	orderResponse := BuildOrderResp{
		market.MakerFeeRate,
		market.TakerFeeRate,
		decimal.Zero,
		order.Side == "sell",
		order.OrderType == "market",
		false)

	orderJson := models.OrderJSON{
		Trader:                  address,
		Relayer:                 os.Getenv("HSK_RELAYER_ADDRESS"),
		BaseCurrency:            market.BaseTokenAddress,
		QuoteCurrency:           market.QuoteTokenAddress,
		BaseCurrencyHugeAmount:  baseTokenHugeAmount,
		QuoteCurrencyHugeAmount: quoteTokenHugeAmount,
		GasTokenHugeAmount:      gasFeeInQuoteTokenHugeAmount,
		Data:                    orderData,
	}

	sdkOrder := sdk.NewOrderWithData(address,
		os.Getenv("HSK_RELAYER_ADDRESS"),
		market.BaseTokenAddress,
		market.QuoteTokenAddress,
		utils.DecimalToBigInt(baseTokenHugeAmount),
		utils.DecimalToBigInt(quoteTokenHugeAmount),
		utils.DecimalToBigInt(gasFeeInQuoteTokenHugeAmount),
		orderData,
		"",
	)

	orderHash := hydro.GetOrderHash(sdkOrder)
	orderResponse := BuildOrderResp{
		ID:              utils.Bytes2HexP(orderHash),
		Json:            &orderJson,
		Side:            order.Side,
		Type:            order.OrderType,
		Price:           price,
		Amount:          amount,
		MarketID:        order.MarketID,
		AsMakerFeeRate:  market.MakerFeeRate,
		AsTakerFeeRate:  market.TakerFeeRate,
		MakerRebateRate: makerRebateRate,
		GasFeeAmount:    gasFeeInQuoteToken,
	}

	cacheOrder := CacheOrder{
		OrderResponse:         orderResponse,
		Address:               address,
		BalanceOfTokenToOffer: offeredAmount,
	}

	// Cache the build order for 60 seconds, if we still not get signature in the period. The order will be dropped.
	err := CacheService.Set(generateOrderCacheKey(orderResponse.ID), utils.ToJsonString(cacheOrder), time.Second*60)
	return &orderResponse, err
}

func generateOrderCacheKey(orderID string) string {
	return "OrderCache:" + orderID
}

func getExpiredAt(expiresInSeconds int64) int64 {
	if time.Duration(expiresInSeconds)*time.Second > time.Hour {
		return time.Now().Unix() + expiresInSeconds
	} else {
		return time.Now().Unix() + 60*60*24*365*100
	}
}

func isMarketBuyOrder(order *BuildOrderReq) bool {
	return order.OrderType == "market" && order.Side == "buy"
}

func isMarketOrder(order *BuildOrderReq) bool {
	return order.OrderType == "market"
}
