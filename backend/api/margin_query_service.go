package api

import (
	"fmt"
	"math/big"
	"net/http"
	// "strconv"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	"github.com/HydroProtocol/hydro-scaffold_dex/backend/sdk_wrappers"
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	// "github.com/ethereum/go-ethereum/common"
	goEthereumCommon "github.com/ethereum/go-ethereum/common"
	"github.com/labstack/echo"
	"github.com/shopspring/decimal"
)

// MarginPositionDetail defines the response structure for a single margin position.
type MarginPositionDetail struct {
	MarketID                  uint16 `json:"marketID"`
	MarketSymbol              string `json:"marketSymbol"`
	Side                      string `json:"side"` // "Long [BaseAsset]", "Short [BaseAsset]", or "Complex"
	Size                      string `json:"size"` // Amount of base asset if clearly long/short, or total debt USD if complex
	CollateralValueUSD        string `json:"collateralValueUSD"`
	DebtValueUSD              string `json:"debtValueUSD"`
	MarginRatio               string `json:"marginRatio"` // e.g., "1.85" for 185%
	IsLiquidatable            bool   `json:"isLiquidatable"`
	AccountStatus             string `json:"accountStatus"`
	EntryPrice                string `json:"entryPrice"`                // Placeholder: "N/A"
	MarkPrice                 string `json:"markPrice"`                 // Placeholder: "N/A" (needs oracle price of base in quote)
	UnrealizedPnL             string `json:"unrealizedPnL"`             // Placeholder: "N/A"
	EstimatedLiquidationPrice string `json:"estimatedLiquidationPrice"` // Placeholder: "N/A"
	BaseAssetSymbol           string `json:"baseAssetSymbol"`
	QuoteAssetSymbol          string `json:"quoteAssetSymbol"`
}

// GetUserMarginPositions handles the request to list a user's margin positions across all markets.
func (h *HydroApi) GetUserMarginPositions(c echo.Context) error {
	customCtx := c.(*CustomContext)
	userAddressHex := customCtx.Get("userAddress").(string)
	if userAddressHex == "" {
		return NewError(http.StatusUnauthorized, "User address not found in context, authentication required.")
	}
	userAddressCommon := goEthereumCommon.HexToAddress(userAddressHex)

	activeMarketIDs, err := models.MarginActivePositionDaoSql.GetActiveMarketsForUser(userAddressHex)
	if err != nil {
		utils.Errorf("GetUserMarginPositions: Error fetching active markets for user %s: %v", userAddressHex, err)
		return NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch active markets: %v", err))
	}

	if len(activeMarketIDs) == 0 {
		return c.JSON(http.StatusOK, []MarginPositionDetail{}) // Return empty list if no active positions
	}

	var results []MarginPositionDetail
	hydroSDK := GetHydroSDK() // Assuming GetHydroSDK() is available in this package/context

	for _, marketIDFromDB := range activeMarketIDs {
		marketID_uint16 := uint16(marketIDFromDB)

		accountDetails, sdkErr := sdk_wrappers.GetAccountDetails(hydroSDK, userAddressCommon, marketID_uint16)
		if sdkErr != nil {
			utils.Warningf("GetUserMarginPositions: Skipping market ID %d for user %s due to GetAccountDetails error: %v", marketID_uint16, userAddressHex, sdkErr)
			continue
		}

		marketDB, dbErr := models.MarketDaoSql.FindMarketByMarketID(marketID_uint16)
		if dbErr != nil {
			utils.Warningf("GetUserMarginPositions: Skipping market ID %d for user %s due to FindMarketByMarketID error: %v", marketID_uint16, userAddressHex, dbErr)
			continue
		}

		baseAssetAddr := goEthereumCommon.HexToAddress(marketDB.BaseTokenAddress)
		quoteAssetAddr := goEthereumCommon.HexToAddress(marketDB.QuoteTokenAddress)

		baseBorrowed, errBaseBorrow := sdk_wrappers.GetAmountBorrowed(hydroSDK, baseAssetAddr, userAddressCommon, marketID_uint16)
		if errBaseBorrow != nil {
			utils.Warningf("GetUserMarginPositions: Error fetching base borrowed for market %d, user %s: %v. Assuming 0.", marketID_uint16, userAddressHex, errBaseBorrow)
			baseBorrowed = big.NewInt(0)
		}
		quoteBorrowed, errQuoteBorrow := sdk_wrappers.GetAmountBorrowed(hydroSDK, quoteAssetAddr, userAddressCommon, marketID_uint16)
		if errQuoteBorrow != nil {
			utils.Warningf("GetUserMarginPositions: Error fetching quote borrowed for market %d, user %s: %v. Assuming 0.", marketID_uint16, userAddressHex, errQuoteBorrow)
			quoteBorrowed = big.NewInt(0)
		}

		detail := MarginPositionDetail{
			MarketID:                  marketID_uint16,
			MarketSymbol:              marketDB.ID, // Using MarketDao.ID as symbol (e.g., "HOT-DAI")
			BaseAssetSymbol:           marketDB.BaseTokenSymbol,
			QuoteAssetSymbol:          marketDB.QuoteTokenSymbol,
			CollateralValueUSD:        utils.BigIntToDecimal(accountDetails.AssetsTotalUSDValue, 18).StringFixed(2),
			DebtValueUSD:              utils.BigIntToDecimal(accountDetails.DebtsTotalUSDValue, 18).StringFixed(2),
			IsLiquidatable:            accountDetails.Liquidatable,
			AccountStatus:             sdk_wrappers.GetAccountStatusString(accountDetails.Status),
			EntryPrice:                "N/A", // Placeholder
			MarkPrice:                 "N/A", // Placeholder
			UnrealizedPnL:             "N/A", // Placeholder
			EstimatedLiquidationPrice: "N/A", // Placeholder
		}

		// Determine Side and Size
		if baseBorrowed.Sign() > 0 && quoteBorrowed.Sign() == 0 {
			detail.Side = fmt.Sprintf("Short %s", marketDB.BaseTokenSymbol)
			detail.Size = utils.BigWeiToDecimal(baseBorrowed, int32(marketDB.BaseTokenDecimals)).StringFixed(marketDB.AmountPrecision)
		} else if quoteBorrowed.Sign() > 0 && baseBorrowed.Sign() == 0 {
			detail.Side = fmt.Sprintf("Long %s", marketDB.BaseTokenSymbol)
			// For long, size is more intuitively the amount of base asset controlled by the position.
			// This requires knowing how much base asset the borrowed quote could buy, or total position value in base terms.
			// For now, representing size as the debt in quote terms as a proxy of leveraged value.
			// A more accurate 'Size' for longs would be (CollateralValueUSD + DebtValueUSD) / BasePriceUSD
			detail.Size = utils.BigWeiToDecimal(quoteBorrowed, int32(marketDB.QuoteTokenDecimals)).StringFixed(marketDB.PricePrecision) + " " + marketDB.QuoteTokenSymbol + " (Debt)"
		} else if baseBorrowed.Sign() > 0 && quoteBorrowed.Sign() > 0 {
			detail.Side = "Complex (Borrowed Both)"
			detail.Size = detail.DebtValueUSD + " USD (Total Debt)"
		} else { // No debt
			if accountDetails.AssetsTotalUSDValue.Sign() > 0 { // Has collateral but no debt
				detail.Side = "No Debt Position" // Or "Idle Collateral"
				detail.Size = "0"
			} else { // No collateral and no debt - this case might be filtered by is_active=TRUE but handle defensively
				utils.Warningf("GetUserMarginPositions: User %s in market %d has no debt and no collateral, but was marked active.", userAddressHex, marketID_uint16)
				continue // Skip this position as it's effectively empty or an anomaly
			}
		}

		// Calculate MarginRatio
		assetsDec := utils.BigIntToDecimal(accountDetails.AssetsTotalUSDValue, 18)
		debtsDec := utils.BigIntToDecimal(accountDetails.DebtsTotalUSDValue, 18)
		if debtsDec.IsPositive() {
			detail.MarginRatio = assetsDec.Div(debtsDec).StringFixed(4)
		} else if assetsDec.IsPositive() && debtsDec.IsZero() {
			detail.MarginRatio = "inf" // Effectively infinite margin
		} else {
			detail.MarginRatio = "0.00" // Or "N/A" if no assets and no debt
		}

		results = append(results, detail)
	}

	return c.JSON(http.StatusOK, results)
}
