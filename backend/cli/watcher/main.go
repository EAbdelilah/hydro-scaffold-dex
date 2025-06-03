package main

import (
	_ "github.com/joho/godotenv/autoload"
)

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/cli"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/connection"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/messagebus"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/sdk_wrappers"
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk/ethereum"
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/HydroProtocol/nights-watch"
	"github.com/HydroProtocol/nights-watch/plugin"
	"github.com/HydroProtocol/nights-watch/structs"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
)

// MarginContractABIJsonString contains a key-feature subset of the full Margin contract ABI for the watcher.
// Developer MUST ensure the true, complete ABI is manually placed here later if this subset is insufficient.
const MarginContractABIJsonString = `[{"constant":true,"inputs":[{"name":"user","type":"address"},{"name":"marketID","type":"uint16"}],"name":"getAccountDetails","outputs":[{"components":[{"name":"liquidatable","type":"bool"},{"name":"status","type":"uint8"},{"name":"debtsTotalUSDValue","type":"uint256"},{"name":"balancesTotalUSDValue","type":"uint256"}],"name":"details","type":"tuple"}],"payable":false,"stateMutability":"view","type":"function"}, {"constant":true,"inputs":[{"name":"asset","type":"address"},{"name":"user","type":"address"},{"name":"marketID","type":"uint16"}],"name":"getAmountBorrowed","outputs":[{"name":"amount","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}, {"constant":true,"inputs":[{"name":"assetAddress","type":"address"}],"name":"getAsset","outputs":[{"components":[{"name":"lendingPoolToken","type":"address"},{"name":"priceOracle","type":"address"},{"name":"interestModel","type":"address"}],"name":"asset","type":"tuple"}],"payable":false,"stateMutability":"view","type":"function"}, {"constant":false,"inputs":[{"components":[{"name":"actionType","type":"uint8"},{"name":"encodedParams","type":"bytes"}],"name":"actions","type":"tuple[]"}],"name":"batch","outputs":[],"payable":true,"stateMutability":"payable","type":"function"}, {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"IncreaseCollateral","type":"event"}, {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"DecreaseCollateral","type":"event"}, {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"Borrow","type":"event"}, {"anonymous":false,"inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"name":"Repay","type":"event"} ]`; // Key-feature subset

const IncreaseCollateralEventSignature = "IncreaseCollateral(address,uint16,address,uint256)"
const DecreaseCollateralEventSignature = "DecreaseCollateral(address,uint16,address,uint256)"
const BorrowEventSignature = "Borrow(address,uint16,address,uint256)"
const RepayEventSignature = "Repay(address,uint16,address,uint256)"

var marginContractABI abi.ABI

type DBTransactionHandler struct {
	eventQueue  common.IQueue
	kvStore     common.IKVStore
	redisClient *connection.RedisClient
}

type MarginContractEventHandler struct {
	redisClient *connection.RedisClient
	hydroSDK    sdk.Hydro
}

func getAccountStatusString(statusUint8 uint8) string {
	switch statusUint8 {
	case 0:
		return "Normal"
	case 1:
		return "MarginCall"
	case 2:
		return "Liquidated"
	default:
		return "Unknown"
	}
}

func (h *MarginContractEventHandler) Handle(logEntry structs.Log) {
	utils.Infof("Received Margin Contract Event: %s, TxHash: %s, Block: %d", logEntry.GetEventName(), logEntry.GetTransactionHash(), logEntry.GetBlockNumber())

	var user ethcommon.Address
	var eventMarketID_uint16 uint16
	var specificAsset ethcommon.Address

	if len(logEntry.GetTopics()) > 2 { // topic[0] is event sig
		user = ethcommon.BytesToAddress(logEntry.GetTopics()[1].Bytes()) // topic[1] is typically user
		eventMarketID_uint16 = uint16(new(big.Int).SetBytes(logEntry.GetTopics()[2].Bytes()).Uint64()) // topic[2] is typically marketID
	} else {
		utils.Errorf("Not enough topics in log entry for event %s, tx %s to extract user and marketID", logEntry.GetEventName(), logEntry.GetTransactionHash())
		return
	}

	if len(logEntry.GetTopics()) > 3 {
		specificAsset = ethcommon.BytesToAddress(logEntry.GetTopics()[3].Bytes()) // topic[3] is typically asset
	}

	var eventData struct{ Amount *big.Int }

	switch logEntry.GetEventName() {
	case "IncreaseCollateral":
		event, ok := marginContractABI.Events["IncreaseCollateral"]
		if !ok { utils.Errorf("IncreaseCollateral event definition not found"); return }
		if err := event.Inputs.NonIndexed().Unpack(&eventData, logEntry.GetData()); err != nil {
			utils.Errorf("Error unpacking IncreaseCollateral data: %v. Tx: %s", err, logEntry.GetTransactionHash()); return
		}
		utils.Infof("IncreaseCollateral: User %s, Market %d, Asset %s, Amount %s", user.Hex(), eventMarketID_uint16, specificAsset.Hex(), eventData.Amount.String())
		currentPos, dbErr := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), eventMarketID_uint16)
		if dbErr != nil {
			utils.Errorf("IncreaseCollateral: GetOrCreate error: %v", dbErr)
		} else {
			if err := models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), eventMarketID_uint16, true, currentPos.HasDebt); err != nil {
				utils.Errorf("IncreaseCollateral: UpdateActivity error: %v", err)
			}
		}

	case "DecreaseCollateral":
		event, ok := marginContractABI.Events["DecreaseCollateral"]
		if !ok { utils.Errorf("DecreaseCollateral event definition not found"); return }
		if err := event.Inputs.NonIndexed().Unpack(&eventData, logEntry.GetData()); err != nil {
			utils.Errorf("Error unpacking DecreaseCollateral data: %v. Tx: %s", err, logEntry.GetTransactionHash()); return
		}
		utils.Infof("DecreaseCollateral: User %s, Market %d, Asset %s, Amount %s", user.Hex(), eventMarketID_uint16, specificAsset.Hex(), eventData.Amount.String())
		marketDBInfo, err := models.MarketDao.FindMarketByMarketID(eventMarketID_uint16)
		if err != nil {
			utils.Errorf("DecreaseCollateral: Error fetching market DB info for market %d: %v", eventMarketID_uint16, err)
		} else {
			baseAssetAddr := ethcommon.HexToAddress(marketDBInfo.BaseTokenAddress)
			quoteAssetAddr := ethcommon.HexToAddress(marketDBInfo.QuoteTokenAddress)
			baseBalance, errBase := sdk_wrappers.MarketBalanceOf(h.hydroSDK, eventMarketID_uint16, baseAssetAddr, user)
			if errBase != nil { utils.Warningf("DecreaseCollateral: Error fetching base balance: %v", errBase); baseBalance = big.NewInt(0) }
			quoteBalance, errQuote := sdk_wrappers.MarketBalanceOf(h.hydroSDK, eventMarketID_uint16, quoteAssetAddr, user)
			if errQuote != nil { utils.Warningf("DecreaseCollateral: Error fetching quote balance: %v", errQuote); quoteBalance = big.NewInt(0) }
			newHasCollateral := (baseBalance.Sign() > 0 || quoteBalance.Sign() > 0)
			utils.Infof("Watcher/DecreaseCollateral: Post-event balances for user %s, market %d - Base: %s, Quote: %s. newHasCollateral: %t", user.Hex(), eventMarketID_uint16, baseBalance.String(), quoteBalance.String(), newHasCollateral)
			currentPos, dbErrGet := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), eventMarketID_uint16)
			if dbErrGet != nil {
				utils.Errorf("DecreaseCollateral: GetOrCreate error: %v", dbErrGet)
			} else {
				if errUpdate := models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), eventMarketID_uint16, newHasCollateral, currentPos.HasDebt); errUpdate != nil {
					utils.Errorf("DecreaseCollateral: UpdateActivity error: %v", errUpdate)
				}
			}
		}

	case "Borrow":
		event, ok := marginContractABI.Events["Borrow"]
		if !ok { utils.Errorf("Borrow event definition not found"); return }
		if err := event.Inputs.NonIndexed().Unpack(&eventData, logEntry.GetData()); err != nil {
			utils.Errorf("Error unpacking Borrow data: %v. Tx: %s", err, logEntry.GetTransactionHash()); return
		}
		utils.Infof("Borrow: User %s, Market %d, Asset %s, Amount %s", user.Hex(), eventMarketID_uint16, specificAsset.Hex(), eventData.Amount.String())
		currentPos, dbErr := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), eventMarketID_uint16)
		if dbErr != nil {
			utils.Errorf("Borrow: GetOrCreate error: %v", dbErr)
		} else {
			if err := models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), eventMarketID_uint16, currentPos.HasCollateral, true); err != nil {
				utils.Errorf("Borrow: UpdateActivity error: %v", err)
			}
		}

	case "Repay":
		event, ok := marginContractABI.Events["Repay"]
		if !ok { utils.Errorf("Repay event definition not found"); return }
		if err := event.Inputs.NonIndexed().Unpack(&eventData, logEntry.GetData()); err != nil {
			utils.Errorf("Error unpacking Repay data: %v. Tx: %s", err, logEntry.GetTransactionHash()); return
		}
		utils.Infof("Repay: User %s, Market %d, Asset %s, Amount %s", user.Hex(), eventMarketID_uint16, specificAsset.Hex(), eventData.Amount.String())
		marketDBInfo, err := models.MarketDao.FindMarketByMarketID(eventMarketID_uint16)
		if err != nil {
			utils.Errorf("Repay: Error fetching market DB info for market %d: %v", eventMarketID_uint16, err)
		} else {
			baseAssetAddr := ethcommon.HexToAddress(marketDBInfo.BaseTokenAddress)
			quoteAssetAddr := ethcommon.HexToAddress(marketDBInfo.QuoteTokenAddress)

			baseDebt, errBaseDebt := sdk_wrappers.GetAmountBorrowed(h.hydroSDK, user, eventMarketID_uint16, baseAssetAddr)
			if errBaseDebt != nil {
				utils.Warningf("Repay: Error fetching base debt for user %s, market %d: %v", user.Hex(), eventMarketID_uint16, errBaseDebt)
				baseDebt = big.NewInt(0)
			}
			quoteDebt, errQuoteDebt := sdk_wrappers.GetAmountBorrowed(h.hydroSDK, user, eventMarketID_uint16, quoteAssetAddr)
			if errQuoteDebt != nil {
				utils.Warningf("Repay: Error fetching quote debt for user %s, market %d: %v", user.Hex(), eventMarketID_uint16, errQuoteDebt)
				quoteDebt = big.NewInt(0)
			}
			newHasDebt := (baseDebt.Sign() > 0 || quoteDebt.Sign() > 0)
			utils.Infof("Watcher/Repay: Post-event debts for user %s, market %d - Base: %s, Quote: %s. newHasDebt: %t", user.Hex(), eventMarketID_uint16, baseDebt.String(), quoteDebt.String(), newHasDebt)

			currentPos, dbErrGet := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), eventMarketID_uint16)
			if dbErrGet != nil {
				utils.Errorf("Repay: GetOrCreate error: %v", dbErrGet)
				// If we can't get/create, we probably can't update, so might as well return or skip DB update
			} else {
				if errUpdate := models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), eventMarketID_uint16, currentPos.HasCollateral, newHasDebt); errUpdate != nil {
					utils.Errorf("Repay: UpdateActivity error: %v", errUpdate)
				}
			}
		}

	default:
		utils.Debugf("Unhandled Margin Contract event type: %s from Tx %s", logEntry.GetEventName(), logEntry.GetTransactionHash())
		return
	}

	userAddressHex := user.Hex()
	accountDetails, accDetailsErr := sdk_wrappers.GetAccountDetails(h.hydroSDK, user, eventMarketID_uint16)
	marketDBDetails, marketDBErr := models.MarketDao.FindMarketByMarketID(eventMarketID_uint16)

	updatePayload := messagebus.MarginAccountUpdateData{
		UserAddress: userAddressHex,
		MarketID:    eventMarketID_uint16,
		Timestamp:   time.Now().Unix(),
	}

	if accDetailsErr == nil && accountDetails != nil {
		updatePayload.Liquidatable = accountDetails.Liquidatable
		updatePayload.Status = getAccountStatusString(accountDetails.Status)
		updatePayload.DebtsTotalUSDValue = accountDetails.DebtsTotalUSDValue.String()
		updatePayload.BalancesTotalUSDValue = accountDetails.AssetsTotalUSDValue.String()
		if accountDetails.DebtsTotalUSDValue != nil && accountDetails.DebtsTotalUSDValue.Sign() > 0 {
			ratio := new(big.Int).Mul(accountDetails.AssetsTotalUSDValue, utils.GetExp(18))
			ratio.Div(ratio, accountDetails.DebtsTotalUSDValue)
			updatePayload.CollateralRatio = ratio.String()
		} else if accountDetails.AssetsTotalUSDValue != nil && accountDetails.AssetsTotalUSDValue.Sign() > 0 {
			updatePayload.CollateralRatio = "inf"
		} else {
			updatePayload.CollateralRatio = "0"
		}
	} else {
		utils.Errorf("Error fetching account details for targeted message (%s event): %v", logEntry.GetEventName(), accDetailsErr)
	}

	if marketDBErr == nil && marketDBDetails != nil {
		baseAssetAddr := ethcommon.HexToAddress(marketDBDetails.BaseTokenAddress)
		quoteAssetAddr := ethcommon.HexToAddress(marketDBDetails.QuoteTokenAddress)

		baseCollateralBal, errBaseBal := sdk_wrappers.MarketBalanceOf(h.hydroSDK, eventMarketID_uint16, baseAssetAddr, user)
		if errBaseBal != nil { utils.Warningf("Watcher/UpdatePayload: Error fetching base balance for user %s, market %d: %v", user.Hex(), eventMarketID_uint16, errBaseBal); baseCollateralBal = big.NewInt(0) }

		quoteCollateralBal, errQuoteBal := sdk_wrappers.MarketBalanceOf(h.hydroSDK, eventMarketID_uint16, quoteAssetAddr, user)
		if errQuoteBal != nil { utils.Warningf("Watcher/UpdatePayload: Error fetching quote balance for user %s, market %d: %v", user.Hex(), eventMarketID_uint16, errQuoteBal); quoteCollateralBal = big.NewInt(0) }

		baseBorrowedAmt, errBaseDebt := sdk_wrappers.GetAmountBorrowed(h.hydroSDK, user, eventMarketID_uint16, baseAssetAddr)
		if errBaseDebt != nil { utils.Warningf("Watcher/UpdatePayload: Error fetching base debt for user %s, market %d: %v", user.Hex(), eventMarketID_uint16, errBaseDebt); baseBorrowedAmt = big.NewInt(0) }

		quoteBorrowedAmt, errQuoteDebt := sdk_wrappers.GetAmountBorrowed(h.hydroSDK, user, eventMarketID_uint16, quoteAssetAddr)
		if errQuoteDebt != nil { utils.Warningf("Watcher/UpdatePayload: Error fetching quote debt for user %s, market %d: %v", user.Hex(), eventMarketID_uint16, errQuoteDebt); quoteBorrowedAmt = big.NewInt(0) }


		var basePriceStr, quotePriceStr = "0", "0"
		if baseAssetAddr != (ethcommon.Address{}) {
			baseAssetContractInfo, gaiErr := sdk_wrappers.GetAsset(h.hydroSDK, baseAssetAddr)
			if gaiErr == nil && baseAssetContractInfo.PriceOracle != (ethcommon.Address{}) {
				if bp, bpErr := sdk_wrappers.GetOraclePrice(h.hydroSDK, baseAssetContractInfo.PriceOracle, baseAssetAddr); bpErr == nil {
					basePriceStr = bp.String()
				} else {
					utils.Warningf("Watcher: Error fetching base asset oracle price for %s from oracle %s: %v", baseAssetAddr.Hex(), baseAssetContractInfo.PriceOracle.Hex(), bpErr)
				}
			} else if gaiErr != nil {
				utils.Warningf("Watcher: Error fetching asset info for base asset %s: %v", baseAssetAddr.Hex(), gaiErr)
			} else {
				utils.Warningf("Watcher: No price oracle address found for base asset %s", baseAssetAddr.Hex())
			}
		}
		if quoteAssetAddr != (ethcommon.Address{}) {
			quoteAssetContractInfo, gaiErr := sdk_wrappers.GetAsset(h.hydroSDK, quoteAssetAddr)
			if gaiErr == nil && quoteAssetContractInfo.PriceOracle != (ethcommon.Address{}) {
				if qp, qpErr := sdk_wrappers.GetOraclePrice(h.hydroSDK, quoteAssetContractInfo.PriceOracle, quoteAssetAddr); qpErr == nil {
					quotePriceStr = qp.String()
				} else {
					utils.Warningf("Watcher: Error fetching quote asset oracle price for %s from oracle %s: %v", quoteAssetAddr.Hex(), quoteAssetContractInfo.PriceOracle.Hex(), qpErr)
				}
			} else if gaiErr != nil {
				utils.Warningf("Watcher: Error fetching asset info for quote asset %s: %v", quoteAssetAddr.Hex(), gaiErr)
			} else {
				utils.Warningf("Watcher: No price oracle address found for quote asset %s", quoteAssetAddr.Hex())
			}
		}

		updatePayload.BaseAssetDetails = messagebus.AssetMarginDetails{
			Address:          baseAssetAddr.Hex(),
			Symbol:           marketDBDetails.BaseTokenSymbol,
			CollateralAmount: utils.BigWeiToDecimal(baseCollateralBal, int32(marketDBDetails.BaseTokenDecimals)).String(),
			BorrowedAmount:   utils.BigWeiToDecimal(baseBorrowedAmt, int32(marketDBDetails.BaseTokenDecimals)).String(),
			CurrentPrice:     basePriceStr,
		}
		updatePayload.QuoteAssetDetails = messagebus.AssetMarginDetails{
			Address:          quoteAssetAddr.Hex(),
			Symbol:           marketDBDetails.QuoteTokenSymbol,
			CollateralAmount: utils.BigWeiToDecimal(quoteCollateralBal, int32(marketDBDetails.QuoteTokenDecimals)).String(),
			BorrowedAmount:   utils.BigWeiToDecimal(quoteBorrowedAmt, int32(marketDBDetails.QuoteTokenDecimals)).String(),
			CurrentPrice:     quotePriceStr,
		}
		if marketDBDetails.LiquidateRate != nil {
			updatePayload.LiquidateRate = marketDBDetails.LiquidateRate.String()
		}
	} else {
		utils.Errorf("Error fetching market DB details for targeted message (%s event): %v", logEntry.GetEventName(), marketDBErr)
	}

	utils.Infof("Watcher: Publishing MARGIN_ACCOUNT_UPDATE for user %s, market %d. Payload: %+v", userAddressHex, eventMarketID_uint16, updatePayload)
	publishErr := messagebus.PublishToUserQueue(h.redisClient.Client, userAddressHex, "MARGIN_ACCOUNT_UPDATE", updatePayload)
	if publishErr != nil {
		utils.Errorf("Error publishing MARGIN_ACCOUNT_UPDATE to user queue (%s event): %v", logEntry.GetEventName(), publishErr)
	}

	refreshPayload := map[string]interface{}{"marketID": eventMarketID_uint16, "reason": strings.ToLower(logEntry.GetEventName())}
	errRefresh := messagebus.PublishToUserQueue(h.redisClient.Client, userAddressHex, "TRIGGER_MARGIN_ACCOUNT_REFRESH", refreshPayload)
	if errRefresh != nil {
		utils.Errorf("Error publishing TRIGGER_MARGIN_ACCOUNT_REFRESH to user queue for user %s, event %s: %v", userAddressHex, logEntry.GetEventName(), errRefresh)
	} else {
		utils.Infof("Published TRIGGER_MARGIN_ACCOUNT_REFRESH for user %s, event %s", userAddressHex, logEntry.GetEventName())
	}
}

func (handler DBTransactionHandler) TxHandlerFunc(txAndReceipt *structs.RemovableTxAndReceipt) {
	tx := txAndReceipt.Tx
	txReceipt := txAndReceipt.Receipt

	launchLog := models.LaunchLogDao.FindByHash(tx.GetHash())
	if launchLog == nil {
		utils.Debugf("Skip useless transaction %s", tx.GetHash())
		return
	}

	if launchLog.Status != common.STATUS_PENDING {
		utils.Infof("LaunchLog is not pending %s, skip", launchLog.Hash.String)
		return
	}

	txResult := txReceipt.GetResult()
	hash := tx.GetHash()
	transaction := models.TransactionDao.FindTransactionByID(launchLog.ItemID)
	utils.Infof("Transaction %s txResult is %+v", tx.GetHash(), txResult)

	var status string
	if txResult {
		status = common.STATUS_SUCCESSFUL
	} else {
		status = common.STATUS_FAILED
	}

	if launchLog.ItemType == "hydroApprove" {
		launchLog.Status = status
		errUpdate := models.LaunchLogDao.UpdateLaunchLog(launchLog)
		if errUpdate != nil {
			panic(errUpdate)
		}
		return
	}

	event := &common.ConfirmTransactionEvent{
		Event: common.Event{
			Type:     common.EventConfirmTransaction,
			MarketID: transaction.MarketID,
		},
		Hash:      hash,
		Status:    status,
		Timestamp: txAndReceipt.TimeStamp,
	}

	bts, _ := json.Marshal(event)
	errQueue := handler.eventQueue.Push(bts)
	if errQueue != nil {
		utils.Errorf("Push event into Queue Error: %v", errQueue)
	}

	handler.kvStore.Set(common.HYDRO_WATCHER_BLOCK_NUMBER_CACHE_KEY, strconv.FormatUint(tx.GetBlockNumber(), 10), 0)
}

func main() {
	ctx, stop := context.WithCancel(context.Background())
	go cli.WaitExitSignal(stop)

	models.Connect(os.Getenv("HSK_DATABASE_URL"))
	redisClient := connection.NewRedisClient(os.Getenv("HSK_REDIS_URL"))

	kvStore, err := common.InitKVStore(&common.RedisKVStoreConfig{
		Ctx:    ctx,
		Client: redisClient,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to init KVStore: %v", err))
	}

	queue, err := common.InitQueue(&common.RedisQueueConfig{
		Name:   common.HYDRO_ENGINE_EVENTS_QUEUE_KEY,
		Client: redisClient,
		Ctx:    ctx,
	})
	if err != nil {
		panic(err)
	}

	filter := func(tx sdk.Transaction) bool {
		return models.LaunchLogDao.FindByHash(tx.GetHash()) != nil
	}

	dbTxHandler := DBTransactionHandler{
		eventQueue: queue,
		kvStore:    kvStore,
	}

	txReceiptPlugin := plugin.NewTxReceiptPluginWithFilter(dbTxHandler.TxHandlerFunc, filter)

	api := os.Getenv("HSK_BLOCKCHAIN_RPC_URL")
	watcher := nights_watch.NewHttpBasedEthWatcher(ctx, api)
	watcher.RegisterTxReceiptPlugin(txReceiptPlugin)

	var errAbi error
	marginContractABI, errAbi = abi.JSON(strings.NewReader(MarginContractABIJsonString))
	if errAbi != nil {
		panic(fmt.Sprintf("Failed to parse MarginContractABIJsonString: %v", errAbi))
	}

	hydroSDKForWatcher := ethereum.NewEthereumHydro(api, "")
	marginEventHandler := &MarginContractEventHandler{
		redisClient: redisClient,
		hydroSDK:    hydroSDKForWatcher,
	}

	marginContractAddressHex := os.Getenv("HSK_MARGIN_CONTRACT_ADDRESS")
	if marginContractAddressHex == "" {
		utils.Warningf("HSK_MARGIN_CONTRACT_ADDRESS is not set. Margin event watcher will not be effective.")
	}

	eventPlugin := plugin.NewContractEventPluginWithFilter(marginEventHandler, marginContractAddressHex, &marginContractABI, nil)
	watcher.RegisterContractEventPlugin(eventPlugin)

	syncedBlockInCache, err := kvStore.Get(common.HYDRO_WATCHER_BLOCK_NUMBER_CACHE_KEY)
	if err != nil && err != common.KVStoreEmpty {
		panic(err)
	}

	var startFromBlock uint64
	if b, convErr := strconv.Atoi(syncedBlockInCache); convErr == nil {
		startFromBlock = uint64(b) + 1
	} else {
		startFromBlock = 0
	}

	go utils.StartMetrics()
	errRun := watcher.RunTillExitFromBlock(startFromBlock)
	if errRun != nil {
		utils.Infof("Watcher Exit with err: %s", errRun)
	} else {
		utils.Infof("Watcher Exit")
	}
}
