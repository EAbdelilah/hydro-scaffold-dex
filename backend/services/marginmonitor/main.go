package main

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/sdk_wrappers" // Assuming this is where GetAccountDetails etc. are
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk/ethereum" // For hydro instance initialization
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" // GORM postgres dialect driver
	"github.com/shopspring/decimal"
	// "github.com/joho/godotenv/autoload" // For local .env loading, if used
)

// MarketInfoForMonitor holds details for a margin-enabled market relevant to the monitor.
type MarketInfoForMonitor struct {
	MarketID              uint16
	BaseAssetAddress      common.Address
	QuoteAssetAddress     common.Address
	LiquidateRate         *big.Int // Scaled by 1e18
	// InitialMarginFraction *big.Int // Scaled by 1e18 - Not used by monitor directly yet
	// MaintenanceMarginFraction *big.Int // Scaled by 1e18 - Not used by monitor directly yet
}

// MarginMonitorService encapsulates the logic for monitoring margin positions.
type MarginMonitorService struct {
	db    *gorm.DB
	hydro sdk.Hydro // Hydro SDK instance for blockchain interactions
	redisClient *connection.RedisClient // For publishing messages

	// Configuration for monitoring logic
	warningThresholdFactor  float64 // e.g., 1.2 (120% of liquidateRate)
	criticalThresholdFactor float64 // e.g., 1.05 (105% of liquidateRate)
	alertCooldown           time.Duration
	significantChangeThreshold float64 // e.g., 0.05 for 5% change in collateral ratio

	// State for monitoring logic
	currentPrices  map[common.Address]*big.Int // assetAddress -> price (scaled by 1e18)
	lastAlerts     map[string]time.Time        // positionKey -> lastAlertTime
	previousStates map[string]PositionState    // positionKey -> previous state
}

// PositionState stores key metrics of a user's margin position for change detection.
type PositionState struct {
	CollateralRatio     *big.Int // Scaled by 1e18
	AssetsTotalUSDValue *big.Int
	DebtsTotalUSDValue  *big.Int
	Timestamp           time.Time
}


// NewMarginMonitorService creates a new instance of MarginMonitorService.
func NewMarginMonitorService(db *gorm.DB, hydro sdk.Hydro, redisClient *connection.RedisClient) *MarginMonitorService {
	return &MarginMonitorService{
		db:            db,
		hydro:         hydro,
		redisClient:   redisClient,
		warningThresholdFactor:  1.2, // Default, consider making configurable
		criticalThresholdFactor: 1.05, // Default
		alertCooldown:           5 * time.Minute, // Default
		significantChangeThreshold: 0.05, // Default
		currentPrices: make(map[common.Address]*big.Int),
		lastAlerts:    make(map[string]time.Time),
		previousStates:make(map[string]PositionState),
	}
}

// formatRatio converts a big.Int ratio (scaled by 1e18) to a percentage string.
func formatRatio(ratio *big.Int) string {
	if ratio == nil {
		return "N/A"
	}
	ratioFloat := new(big.Float).SetInt(ratio)
	ratioFloat.Quo(ratioFloat, new(big.Float).SetInt(utils.GetExp(16))) // Convert to percentage (div by 1e16 as 1e18 is 100%)
	return fmt.Sprintf("%.2f%%", ratioFloat)
}


// getMarginEnabledMarketsWithDetailsFromDB fetches all margin-enabled markets and their relevant parameters.
func (s *MarginMonitorService) getMarginEnabledMarketsWithDetailsFromDB() ([]MarketInfoForMonitor, error) {
	var marketsFromDB []models.Market
	if err := s.db.Where("borrow_enable = ?", true).Find(&marketsFromDB).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []MarketInfoForMonitor{}, nil // No margin markets found is not an error
		}
		utils.Errorf("Error fetching margin enabled markets: %v", err)
		return nil, err
	}

	var marketsForMonitor []MarketInfoForMonitor
	for _, dbMarket := range marketsFromDB {
		marketInfo := MarketInfoForMonitor{
			MarketID:          dbMarket.ID, // models.Market.ID is already uint16
			BaseAssetAddress:  common.HexToAddress(dbMarket.BaseTokenAddress),
			QuoteAssetAddress: common.HexToAddress(dbMarket.QuoteTokenAddress),
		}

		// Parse LiquidateRate (assuming it's stored as a string like "1.1")
		// and scale it to 1e18.
		if dbMarket.LiquidateRate.String() != "" { // Check if the decimal field is set
			// models.Market.LiquidateRate is decimal.Decimal
			// We need to convert decimal.Decimal to *big.Int scaled by 1e18
			// 1e18 as a decimal
			scaleFactor := decimal.NewFromBigInt(big.NewInt(1), 18)
			scaledRate := dbMarket.LiquidateRate.Mul(scaleFactor)
			marketInfo.LiquidateRate = scaledRate.BigInt()
		} else {
			// Default or handle error if liquidate_rate is crucial and missing
			utils.Warningf("Market %s (ID: %d) has no liquidate_rate set.", dbMarket.Name, dbMarket.ID)
			// Set a default or skip this market if rate is mandatory. For now, let it be nil.
			marketInfo.LiquidateRate = big.NewInt(0) // Or some other default
		}
		marketsForMonitor = append(marketsForMonitor, marketInfo)
	}

	utils.Infof("Fetched %d margin-enabled markets for monitoring.", len(marketsForMonitor))
	return marketsForMonitor, nil
}

// getActiveUsersForMarket fetches all users with active positions in a given market.
func (s *MarginMonitorService) getActiveUsersForMarket(marketID uint16) ([]common.Address, error) {
	var activePositions []models.MarginActivePosition
	err := s.db.Where("market_id = ? AND is_active = ?", marketID, true).Find(&activePositions).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []common.Address{}, nil // No active users is not an error
		}
		utils.Errorf("Error fetching active users for market ID %d: %v", marketID, err)
		return nil, err
	}

	var userAddresses []common.Address
	for _, position := range activePositions {
		userAddresses = append(userAddresses, common.HexToAddress(position.UserAddress))
	}

	utils.Infof("Found %d active users for market ID %d.", len(userAddresses), marketID)
	return userAddresses, nil
}

// monitorPositions is the core loop that checks user positions.
func (s *MarginMonitorService) monitorPositions() {
	utils.Info("MarginMonitorService: Starting position monitoring cycle.")

	// 1. Fetch all oracle prices for relevant assets in this cycle
	s.currentPrices = make(map[common.Address]*big.Int) // Clear previous prices
	allAssetsToPrice := make(map[common.Address]bool)

	markets, err := s.getMarginEnabledMarketsWithDetailsFromDB()
	if err != nil {
		utils.Errorf("MarginMonitorService: Failed to get markets: %v", err)
		return
	}
	if len(markets) == 0 {
		utils.Info("MarginMonitorService: No margin-enabled markets found to monitor.")
		return
	}

	for _, market := range markets {
		allAssetsToPrice[market.BaseAssetAddress] = true
		allAssetsToPrice[market.QuoteAssetAddress] = true
	}

	for assetAddr := range allAssetsToPrice {
		assetInfo, gaiErr := sdk_wrappers.GetAsset(s.hydro, assetAddr)
		if gaiErr == nil && assetInfo.PriceOracle != (common.Address{}) {
			price, priceErr := sdk_wrappers.GetOraclePrice(s.hydro, assetInfo.PriceOracle, assetAddr)
			if priceErr == nil {
				s.currentPrices[assetAddr] = price
				utils.Debugf("Fetched price for asset %s: %s", assetAddr.Hex(), price.String())
			} else {
				utils.Warningf("Could not fetch price for asset %s from oracle %s: %v", assetAddr.Hex(), assetInfo.PriceOracle.Hex(), priceErr)
				s.currentPrices[assetAddr] = big.NewInt(0) // Default to 0 if price fetch fails
			}
		} else if gaiErr != nil {
			utils.Warningf("Could not get asset info (for oracle) for asset %s: %v", assetAddr.Hex(), gaiErr)
			s.currentPrices[assetAddr] = big.NewInt(0)
		} else {
			utils.Warningf("No price oracle found for asset %s", assetAddr.Hex())
			s.currentPrices[assetAddr] = big.NewInt(0)
		}
	}
	utils.Infof("Fetched %d oracle prices for current monitoring cycle.", len(s.currentPrices))


	for _, market := range markets {
		utils.Debugf("MarginMonitorService: Checking market %d (%s-%s)", market.MarketID, market.BaseAssetAddress.Hex(), market.QuoteAssetAddress.Hex())
		activeUsers, err := s.getActiveUsersForMarket(market.MarketID)
		if err != nil {
			utils.Errorf("MarginMonitorService: Failed to get active users for market %d: %v", market.MarketID, err)
			continue
		}

		for _, userAddress := range activeUsers {
			positionKey := fmt.Sprintf("%s-%d", userAddress.Hex(), market.MarketID)
			utils.Debugf("MarginMonitorService: Evaluating user %s in market %d", userAddress.Hex(), market.MarketID)

			accountDetails, sdkErr := sdk_wrappers.GetAccountDetails(s.hydro, userAddress, market.MarketID)
			if sdkErr != nil {
				utils.Errorf("MarginMonitorService: Failed to get account details for user %s, market %d: %v", userAddress.Hex(), market.MarketID, sdkErr)
				continue
			}

			// Calculate Current Collateral Ratio
			var currentCollateralRatio *big.Int
			if accountDetails.DebtsTotalUSDValue != nil && accountDetails.DebtsTotalUSDValue.Sign() > 0 {
				currentCollateralRatio = new(big.Int).Mul(accountDetails.AssetsTotalUSDValue, utils.GetExp(18))
				currentCollateralRatio.Div(currentCollateralRatio, accountDetails.DebtsTotalUSDValue)
			} else if accountDetails.AssetsTotalUSDValue != nil && accountDetails.AssetsTotalUSDValue.Sign() > 0 {
				// No debt, or zero debt, and positive assets: ratio is effectively infinite.
		// Using a very large number (100 * 1e18, effectively 10000% if 1e18 is 100%)
				currentCollateralRatio = new(big.Int).Lsh(utils.GetExp(18), 7) // utils.GetExp(18) is 1e18. Lsh by 7 means multiply by 2^7 = 128. So 128 * 1e18.
                                                                         // This is a common way to represent a very large ratio. MaxAmount might be too large for some display purposes.
			} else {
				// No assets and no debt, or no assets and some debt (though latter implies already liquidated or error state)
				currentCollateralRatio = big.NewInt(0)
			}

			utils.Infof("MarginMonitor: User %s, MarketID %d - AssetsUSD: %s, DebtsUSD: %s, Calculated Ratio (1e18): %s",
				userAddress.Hex(), market.ID, accountDetails.AssetsTotalUSDValue.String(), accountDetails.DebtsTotalUSDValue.String(), currentCollateralRatio.String())


			// Calculate Thresholds
			liquidateRateFromMarket := market.LiquidateRate
			if liquidateRateFromMarket == nil || liquidateRateFromMarket.Sign() == 0 {
				utils.Warningf("Market %d has invalid liquidateRate (%v), skipping alert checks for this market.", market.MarketID, liquidateRateFromMarket)
				continue
			}
			warningThreshBigFloat := new(big.Float).Mul(new(big.Float).SetInt(liquidateRateFromMarket), big.NewFloat(s.warningThresholdFactor))
			criticalThreshBigFloat := new(big.Float).Mul(new(big.Float).SetInt(liquidateRateFromMarket), big.NewFloat(s.criticalThresholdFactor))
			warningThresholdCollateralRatio, _ := warningThreshBigFloat.Int(nil)
			criticalThresholdCollateralRatio, _ := criticalThreshBigFloat.Int(nil)

			// Alerting Logic
			now := time.Now()
			lastAlertTime, found := s.lastAlerts[positionKey]
			alertable := !found || now.Sub(lastAlertTime) > s.alertCooldown

			alertMsgPayload := messagebus.MarginAlertData{
				UserAddress:     userAddress.Hex(),
				MarketID:        market.MarketID,
				CurrentRatio:    currentCollateralRatio.String(), // Send raw scaled value
				LiquidateRate:   liquidateRateFromMarket.String(),
				WarningThreshold: warningThresholdCollateralRatio.String(),
				CriticalThreshold: criticalThresholdCollateralRatio.String(),
				Timestamp:       now.Unix(),
			}

			if currentCollateralRatio.Cmp(criticalThresholdCollateralRatio) < 0 {
				alertMsgPayload.Level = "CRITICAL"
				alertMsgPayload.Message = fmt.Sprintf("CRITICAL: Margin ratio %s is below critical threshold %s!", formatRatio(currentCollateralRatio), formatRatio(criticalThresholdCollateralRatio))
				utils.Warningf(alertMsgPayload.Message + " User: %s, Market: %d", userAddress.Hex(), market.MarketID)
				if alertable {
					messagebus.PublishToUserQueue(s.redisClient.Client, userAddress.Hex(), "MARGIN_ALERT", alertMsgPayload)
					s.lastAlerts[positionKey] = now
				}
			} else if currentCollateralRatio.Cmp(warningThresholdCollateralRatio) < 0 {
				alertMsgPayload.Level = "WARNING"
				alertMsgPayload.Message = fmt.Sprintf("WARNING: Margin ratio %s is below warning threshold %s.", formatRatio(currentCollateralRatio), formatRatio(warningThresholdCollateralRatio))
				utils.Warningf(alertMsgPayload.Message + " User: %s, Market: %d", userAddress.Hex(), market.MarketID)
				if alertable {
					messagebus.PublishToUserQueue(s.redisClient.Client, userAddress.Hex(), "MARGIN_ALERT", alertMsgPayload)
					s.lastAlerts[positionKey] = now
				}
			} else {
				// Healthy or recovering
				if found { // If previously alerted, send a "HEALTHY" update
					alertMsgPayload.Level = "HEALTHY"
					alertMsgPayload.Message = fmt.Sprintf("Margin ratio %s is now healthy.", formatRatio(currentCollateralRatio))
					utils.Infof(alertMsgPayload.Message + " User: %s, Market: %d", userAddress.Hex(), market.MarketID)
					messagebus.PublishToUserQueue(s.redisClient.Client, userAddress.Hex(), "MARGIN_ALERT", alertMsgPayload)
					delete(s.lastAlerts, positionKey) // Clear last alert time as it's healthy
				}
			}

			// Detailed Account Update Publishing (Significant Change or Initial)
			previousState, prevStateExists := s.previousStates[positionKey]
			ratioDiff := big.NewFloat(0)
			if prevStateExists && previousState.CollateralRatio != nil && currentCollateralRatio != nil && currentCollateralRatio.Sign() > 0 { // Avoid division by zero for diff
				diff := new(big.Int).Sub(currentCollateralRatio, previousState.CollateralRatio)
				ratioDiff = new(big.Float).Quo(new(big.Float).SetInt(diff), new(big.Float).SetInt(currentCollateralRatio))
				ratioDiff.Abs(ratioDiff) // Absolute change
			}

			significantChangeThresholdBigFloat := big.NewFloat(s.significantChangeThreshold)
			hasSignificantChange := ratioDiff.Cmp(significantChangeThresholdBigFloat) >= 0

			if !prevStateExists || hasSignificantChange || now.Sub(previousState.Timestamp) > (s.alertCooldown*2) { // Force update if older than 2 cooldowns
				marketDBDetails, marketDBErr := models.MarketDao.FindMarketByMarketID(market.MarketID)
				if marketDBErr != nil {
					utils.Errorf("Error fetching market DB details for MARGIN_ACCOUNT_UPDATE: %v", marketDBErr)
					// Continue without some details if marketDBDetails fails
				}

				baseCollateralBal, errBaseBal := sdk_wrappers.MarketBalanceOf(s.hydro, market.MarketID, market.BaseAssetAddress, userAddress)
				if errBaseBal != nil { utils.Warningf("MarginMonitor/UpdatePayload: Error fetching base balance for user %s, market %d: %v", userAddress.Hex(), market.MarketID, errBaseBal); baseCollateralBal = big.NewInt(0) }

				quoteCollateralBal, errQuoteBal := sdk_wrappers.MarketBalanceOf(s.hydro, market.MarketID, market.QuoteAssetAddress, userAddress)
				if errQuoteBal != nil { utils.Warningf("MarginMonitor/UpdatePayload: Error fetching quote balance for user %s, market %d: %v", userAddress.Hex(), market.MarketID, errQuoteBal); quoteCollateralBal = big.NewInt(0) }

				baseBorrowedAmt, errBaseDebt := sdk_wrappers.GetAmountBorrowed(s.hydro, market.BaseAssetAddress, userAddress, market.MarketID)
				if errBaseDebt != nil { utils.Warningf("MarginMonitor/UpdatePayload: Error fetching base debt for user %s, market %d: %v", userAddress.Hex(), market.MarketID, errBaseDebt); baseBorrowedAmt = big.NewInt(0) }

				quoteBorrowedAmt, errQuoteDebt := sdk_wrappers.GetAmountBorrowed(s.hydro, market.QuoteAssetAddress, userAddress, market.MarketID)
				if errQuoteDebt != nil { utils.Warningf("MarginMonitor/UpdatePayload: Error fetching quote debt for user %s, market %d: %v", userAddress.Hex(), market.MarketID, errQuoteDebt); quoteBorrowedAmt = big.NewInt(0) }

				basePriceStr := "0"
				if price, ok := s.currentPrices[market.BaseAssetAddress]; ok { basePriceStr = price.String() }
				quotePriceStr := "0"
				if price, ok := s.currentPrices[market.QuoteAssetAddress]; ok { quotePriceStr = price.String() }

				updatePayload := messagebus.MarginAccountUpdateData{
					UserAddress:            userAddress.Hex(),
					MarketID:               market.MarketID,
					Timestamp:              now.Unix(),
					Liquidatable:           accountDetails.Liquidatable,
					Status:                 sdk_wrappers.GetAccountStatusString(accountDetails.Status),
					DebtsTotalUSDValue:     accountDetails.DebtsTotalUSDValue.String(),
					BalancesTotalUSDValue:  accountDetails.AssetsTotalUSDValue.String(),
					CollateralRatio:        currentCollateralRatio.String(),
				}
				if marketDBDetails != nil {
					updatePayload.LiquidateRate = marketDBDetails.LiquidateRate.String()
					updatePayload.BaseAssetDetails = messagebus.AssetMarginDetails{
						Address: market.BaseAssetAddress.Hex(), Symbol: marketDBDetails.BaseTokenSymbol,
						CollateralAmount: utils.BigWeiToDecimal(baseCollateralBal, int32(marketDBDetails.BaseTokenDecimals)).String(),
						BorrowedAmount:   utils.BigWeiToDecimal(baseBorrowedAmt, int32(marketDBDetails.BaseTokenDecimals)).String(),
						CurrentPrice:     basePriceStr,
					}
					updatePayload.QuoteAssetDetails = messagebus.AssetMarginDetails{
						Address: market.QuoteAssetAddress.Hex(), Symbol: marketDBDetails.QuoteTokenSymbol,
						CollateralAmount: utils.BigWeiToDecimal(quoteCollateralBal, int32(marketDBDetails.QuoteTokenDecimals)).String(),
						BorrowedAmount:   utils.BigWeiToDecimal(quoteBorrowedAmt, int32(marketDBDetails.QuoteTokenDecimals)).String(),
						CurrentPrice:     quotePriceStr,
					}
				}
				messagebus.PublishToUserQueue(s.redisClient.Client, userAddress.Hex(), "MARGIN_ACCOUNT_UPDATE", updatePayload)
				utils.Infof("Published MARGIN_ACCOUNT_UPDATE for user %s, market %d (Significant Change or Initial)", userAddress.Hex(), market.MarketID)
				s.previousStates[positionKey] = PositionState{
					CollateralRatio: currentCollateralRatio,
					AssetsTotalUSDValue: accountDetails.AssetsTotalUSDValue,
					DebtsTotalUSDValue: accountDetails.DebtsTotalUSDValue,
					Timestamp: now,
				}
			}

			// Actual Liquidation Trigger (based on contract's direct liquidatable flag)
			if accountDetails.Liquidatable {
				// Refined Liquidation Logging
				utils.Warningf("MarginMonitor: User %s in market ID %d IS LIQUIDATABLE. Account Status: %s, Calculated Ratio: %s. AssetsUSD: %s, DebtsUSD: %s. LiquidateRateFactor: %s",
					userAddress.Hex(),
					market.MarketID, // MarketSymbol is not in MarketInfoForMonitor, using ID
					sdk_wrappers.GetAccountStatusString(accountDetails.Status),
					formatRatio(currentCollateralRatio),
					accountDetails.AssetsTotalUSDValue.String(),
					accountDetails.DebtsTotalUSDValue.String(),
					market.LiquidateRate.String(), // This is the factor like 1.1e18
				)

				// Implement Liquidation Action Placeholder
				// TODO: Step 1: Check if an auction for this user/market is already in progress or recently concluded.
				// This might involve querying an internal state (e.g., a local cache of active auctions being processed by this monitor instance)
				// or a new SDK wrapper: e.g., isActiveAuction, err := sdk_wrappers.IsAuctionActiveForAccount(s.hydro, userAddress, market.MarketID)
				// if err != nil {
				//     utils.Errorf("MarginMonitor: Error checking for active auction for user %s, market %d: %v", userAddress.Hex(), market.MarketID, err)
				//     // Decide if to proceed or wait if check fails. For now, assume we might proceed cautiously.
				// } else if isActiveAuction {
				//     utils.Infof("MarginMonitor: Auction already active or recently concluded for user %s, market %d. Skipping new liquidation trigger.", userAddress.Hex(), market.MarketID)
				//     return // or continue to next user, depending on design
				// }
				utils.Infof("MarginMonitor: Conceptually, no active auction found for user %s, market %d. Proceeding with liquidation consideration.", userAddress.Hex(), market.MarketID)

				utils.Infof("MarginMonitor: TODO - Step 2: Prepare conceptual liquidation transaction for user %s, market %d.", userAddress.Hex(), market.MarketID)
				// // 2.1. Create SDKBatchAction for liquidateAccount
				// //      - This would require a new sdk_wrappers.EncodeLiquidateAccountParamsForBatch(userAddress, market.MarketID)
				// //      - Or, if liquidateAccount is not part of batch, a direct sdk_wrapper.PrepareLiquidateAccountTransaction(...)
				// conceptualEncodedLiquidateParams, encErr := sdk_wrappers.EncodeLiquidateAccountParamsForBatch(userAddress, market.MarketID) // This function needs to be created
				// if encErr != nil {
				//     utils.Errorf("MarginMonitor: Conceptually failed to encode liquidate params for user %s, market %d: %v", userAddress.Hex(), market.MarketID, encErr)
				//     // return or continue
				// }
				// actions := []sdk_wrappers.SDKBatchAction{
				//     {
				//         ActionType:    sdk_wrappers.SDKActionTypeLiquidate, // SDKActionTypeLiquidate NEEDS TO BE ADDED to SDK Wrappers
				//         EncodedParams: conceptualEncodedLiquidateParams,
				//     },
				// }

				// // 2.2. Get Unsigned Transaction Data (using a pre-configured liquidator EOA)
				// liquidatorAddressStr := os.Getenv("HSK_LIQUIDATOR_ADDRESS")
				// if liquidatorAddressStr == "" {
				//      utils.Errorf("MarginMonitor: HSK_LIQUIDATOR_ADDRESS is not set. Cannot prepare liquidation tx.")
				//      // return or continue
				// }
				// liquidatorAddress := common.HexToAddress(liquidatorAddressStr)
				// unsignedTxData, prepErr := sdk_wrappers.PrepareBatchActionsTransaction(s.hydro, actions, liquidatorAddress, big.NewInt(0))
				// if prepErr != nil {
				//     utils.Errorf("MarginMonitor: Conceptually failed to prepare liquidation tx for user %s, market %d: %v", userAddress.Hex(), market.MarketID, prepErr)
				//      // return or continue
				// }

				// // 3. Sign and Broadcast (if monitor handles this directly) OR Publish to dedicated queue
				utils.Warningf("MarginMonitor: TODO - Step 3: Actually sign and broadcast liquidation for user %s, market %d, OR publish to dedicated liquidation queue.", userAddress.Hex(), market.MarketID)
				// // Example if publishing:
				// // liquidationTask := map[string]interface{}{"user": userAddress.Hex(), "marketID": market.MarketID, "unsignedTxData": unsignedTxData} // Or just user/marketID
				// // errPublish := messagebus.PublishToQueue(s.redisClient.Client, "LIQUIDATION_JOB_QUEUE", liquidationTask) // Example queue name
				// // if errPublish != nil {
				// //    utils.Errorf("MarginMonitor: Failed to publish liquidation task for user %s, market %d: %v", userAddress.Hex(), market.MarketID, errPublish)
				// // } else {
				// //    utils.Infof("MarginMonitor: Published liquidation task for user %s, market %d to queue.", userAddress.Hex(), market.MarketID)
				// // }
			}
		}
	}
	utils.Info("MarginMonitorService: Finished position monitoring cycle.")
}

func main() {
	utils.Info("Starting Margin Monitor Service...")
	// godotenv.Load() // Load .env file if used for local development

	dbDSN := os.Getenv("HSK_DATABASE_URL")
	if dbDSN == "" {
		panic("HSK_DATABASE_URL environment variable is not set.")
	}
	db, err := gorm.Open("postgres", dbDSN)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}
	defer db.Close()
	models.Init(db) // Initialize gorm global `db` var in models package

	rpcURL := os.Getenv("HSK_BLOCKCHAIN_RPC_URL")
	// The contract address for hydro instance is not strictly needed if sdk_wrappers handle contract addresses internally
	// based on initialized MarginContractAddress.
	hydroInstance := ethereum.NewEthereumHydro(rpcURL, "") // Main exchange address not needed here for sdk_wrappers calls

	// Initialize SDK Wrappers (important for MarginContractAddress and ABI)
	marginContractAddress := os.Getenv("HSK_MARGIN_CONTRACT_ADDRESS")
	if marginContractAddress == "" {
		panic("HSK_MARGIN_CONTRACT_ADDRESS environment variable is not set for monitor.")
	}
	// Pass empty string for hydroContractAddressHex if only margin contract is used by wrappers in this service
	if err := sdk_wrappers.InitHydroWrappers("", marginContractAddress); err != nil {
		panic(fmt.Sprintf("Failed to initialize Hydro SDK Wrappers: %v", err))
	}

	redisUrl := os.Getenv("HSK_REDIS_URL")
	if redisUrl == "" {
		panic("HSK_REDIS_URL environment variable is not set for monitor.")
	}
	redisClient := connection.NewRedisClient(redisUrl)


	monitorService := NewMarginMonitorService(models.GetDB(), hydroInstance, redisClient)

	// TODO: Implement configuration for monitoring interval from env or config file
	monitoringIntervalStr := os.Getenv("MARGIN_MONITOR_INTERVAL_SECONDS")
	monitoringInterval := 1 * time.Minute // Default
	if sec, err := strconv.Atoi(monitoringIntervalStr); err == nil && sec > 0 {
		monitoringInterval = time.Duration(sec) * time.Second
	}
	utils.Infof("Margin Monitor Service: Monitoring interval set to %v", monitoringInterval)
	ticker := time.NewTicker(monitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			monitorService.monitorPositions()
		// TODO: Add a way to gracefully shut down the service (e.g., context cancellation)
		}
	}
}
