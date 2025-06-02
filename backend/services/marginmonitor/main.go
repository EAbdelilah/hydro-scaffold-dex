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
}

// NewMarginMonitorService creates a new instance of MarginMonitorService.
func NewMarginMonitorService(db *gorm.DB, hydro sdk.Hydro) *MarginMonitorService {
	return &MarginMonitorService{db: db, hydro: hydro}
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
	markets, err := s.getMarginEnabledMarketsWithDetailsFromDB()
	if err != nil {
		utils.Errorf("MarginMonitorService: Failed to get markets: %v", err)
		return
	}

	for _, market := range markets {
		utils.Debugf("MarginMonitorService: Checking market %d (%s-%s)", market.MarketID, market.BaseAssetAddress.Hex(), market.QuoteAssetAddress.Hex())
		activeUsers, err := s.getActiveUsersForMarket(market.MarketID)
		if err != nil {
			utils.Errorf("MarginMonitorService: Failed to get active users for market %d: %v", market.MarketID, err)
			continue
		}

		for _, userAddress := range activeUsers {
			utils.Debugf("MarginMonitorService: Checking user %s in market %d", userAddress.Hex(), market.MarketID)

			// 1. Fetch account details using sdk_wrappers
			accountDetails, sdkErr := sdk_wrappers.GetAccountDetails(s.hydro, userAddress, market.MarketID)
			if sdkErr != nil {
				utils.Errorf("MarginMonitorService: Failed to get account details for user %s, market %d: %v", userAddress.Hex(), market.MarketID, sdkErr)
				continue
			}

			// 2. Perform liquidation check
			// accountDetails.Liquidatable is directly from contract.
			// Or, if we want to do it off-chain:
			// assetsUSD := accountDetails.AssetsTotalUSDValue (already *big.Int)
			// debtsUSD := accountDetails.DebtsTotalUSDValue (already *big.Int)
			// liquidateRate := market.LiquidateRate (already *big.Int, scaled by 1e18)
			//
			// if debtsUSD.Cmp(big.NewInt(0)) > 0 {
			//     // requiredCollateral = debtsUSD * liquidateRate / 1e18 (if liquidateRate is factor like 1.1e18)
			//     requiredCollateral := new(big.Int).Mul(debtsUSD, liquidateRate)
			//     requiredCollateral.Div(requiredCollateral, utils.GetExp(18)) // Scale down if liquidateRate is 1e18 scaled
			//
			//     if assetsUSD.Cmp(requiredCollateral) < 0 {
			//         utils.Warningf("User %s in market %d is liquidatable! Assets: %s, Required: %s (Debts: %s, Rate: %s)",
			//             userAddress.Hex(), market.MarketID, assetsUSD.String(), requiredCollateral.String(), debtsUSD.String(), liquidateRate.String())
			//         // TODO: Trigger liquidation process
			//     }
			// }

			if accountDetails.Liquidatable {
				utils.Warningf("User %s in market %d IS LIQUIDATABLE according to contract. AssetsUSD: %s, DebtsUSD: %s, Status: %d",
					userAddress.Hex(), market.MarketID, 
					accountDetails.AssetsTotalUSDValue.String(), 
					accountDetails.DebtsTotalUSDValue.String(), 
					accountDetails.Status)
				// TODO: Trigger liquidation process (e.g., by sending a message to a liquidation bot or an admin system)
				// This might involve preparing a specific type of transaction or calling an admin function.
			} else {
				utils.Debugf("User %s in market %d is NOT liquidatable. AssetsUSD: %s, DebtsUSD: %s, Status: %d",
				userAddress.Hex(), market.MarketID, 
				accountDetails.AssetsTotalUSDValue.String(), 
				accountDetails.DebtsTotalUSDValue.String(), 
				accountDetails.Status)
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
	// HSK_HYBRID_EXCHANGE_ADDRESS is for the original Hydro exchange contract (can be empty if not used by wrappers called here).
	// HSK_MARGIN_CONTRACT_ADDRESS is crucial.
	marginContractAddress := os.Getenv("HSK_MARGIN_CONTRACT_ADDRESS")
	if marginContractAddress == "" {
		panic("HSK_MARGIN_CONTRACT_ADDRESS environment variable is not set for monitor.")
	}
	if err := sdk_wrappers.InitHydroWrappers(os.Getenv("HSK_HYBRID_EXCHANGE_ADDRESS"), marginContractAddress); err != nil {
		panic(fmt.Sprintf("Failed to initialize Hydro SDK Wrappers: %v", err))
	}


	monitorService := NewMarginMonitorService(models.GetDB(), hydroInstance) // Use models.GetDB() to get the global GORM db instance

	// TODO: Implement configuration for monitoring interval
	ticker := time.NewTicker(1 * time.Minute) // Example: monitor every 1 minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			monitorService.monitorPositions()
		// TODO: Add a way to gracefully shut down the service (e.g., context cancellation)
		}
	}
}
