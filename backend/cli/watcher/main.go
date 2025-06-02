package main

import (
	_ "github.com/joho/godotenv/autoload"
)

import (
	"context"
	"encoding/json"
	"github.com/HydroProtocol/nights-watch/plugin"
	"github.com/HydroProtocol/nights-watch/structs"
	"strconv"

	"github.com/HydroProtocol/hydro-scaffold-dex/backend/cli"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/connection"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk"
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/HydroProtocol/nights-watch"
	"os"
	"strings" // For strings.NewReader
	"github.com/ethereum/go-ethereum/accounts/abi" // For ABI parsing
	ethcommon "github.com/ethereum/go-ethereum/common" // For HexToAddress
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/messagebus" // For message publishing
	// Using standard log for watcher, but can be replaced with logrus or similar
)

// TODO: REPLACE THIS WITH THE FULL ABI JSON STRING FOR THE NEW MARGIN CONTRACT
const MarginContractABIJsonString = `[{"constant":true,"inputs":[],"name":"getAccountDetails","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}, {"name":"batch","inputs":[{"components":[{"name":"actionType","type":"uint8"},{"name":"encodedParams","type":"bytes"}],"name":"actions","type":"tuple[]"}],"outputs":[],"payable":true,"stateMutability":"payable","type":"function"}, {"name":"IncreaseCollateral","inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"anonymous":false,"type":"event"}, {"name":"DecreaseCollateral","inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"anonymous":false,"type":"event"}, {"name":"Borrow","inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"anonymous":false,"type":"event"}, {"name":"Repay","inputs":[{"indexed":true,"name":"user","type":"address"},{"indexed":true,"name":"marketID","type":"uint16"},{"indexed":true,"name":"asset","type":"address"},{"indexed":false,"name":"amount","type":"uint256"}],"anonymous":false,"type":"event"}]` // Simplified placeholder + new events

const IncreaseCollateralEventSignature = "IncreaseCollateral(address,uint16,address,uint256)"
const DecreaseCollateralEventSignature = "DecreaseCollateral(address,uint16,address,uint256)"
const BorrowEventSignature = "Borrow(address,uint16,address,uint256)"
const RepayEventSignature = "Repay(address,uint16,address,uint256)"

var marginContractABI abi.ABI

type DBTransactionHandler struct {
	eventQueue common.IQueue
	kvStore    common.IKVStore
	redisClient *connection.RedisClient // Added redisClient for message publishing
}

// MarginContractEventHandler handles events emitted by the Margin Contract.
type MarginContractEventHandler struct {
	redisClient *connection.RedisClient
	// Add other dependencies like DB access if needed directly, or use services.
}

func (h *MarginContractEventHandler) Handle(logEntry structs.Log) {
	utils.Infof("Received Margin Contract Event: %s, TxHash: %s", logEntry.GetEventName(), logEntry.GetTransactionHash())

	var userAddressHex string // To store the user address string for publishing messages

	switch logEntry.GetEventName() {
	case "IncreaseCollateral":
		event, ok := marginContractABI.Events["IncreaseCollateral"]
		if !ok {
			utils.Errorf("IncreaseCollateral event definition not found in ABI")
			return
		}
		// Unpack event data
		// Topics: user (address), marketID (uint16), asset (address)
		// Data: amount (uint256)
		user := ethcommon.BytesToAddress(logEntry.GetTopics()[1].Bytes())
		marketID := new(big.Int).SetBytes(logEntry.GetTopics()[2].Bytes()).Uint64() // marketID is uint16
		asset := ethcommon.BytesToAddress(logEntry.GetTopics()[3].Bytes())
		
		var amount *big.Int
		
		var decodedData struct { Amount *big.Int } // Struct to hold non-indexed params
		err := event.Inputs.NonIndexed().Unpack(&decodedData, logEntry.GetData())
		if err != nil {
			utils.Errorf("Error unpacking IncreaseCollateral event data: %v. Tx: %s. Data: %x", err, logEntry.GetTransactionHash(), logEntry.GetData())
			return
		}
		amount := decodedData.Amount

		utils.Infof("IncreaseCollateral Event: User: %s, MarketID: %d, Asset: %s, Amount: %s",
			user.Hex(), marketID, asset.Hex(), amount.String())
		userAddressHex = user.Hex()

		// TODO: DAO Usage Outline for IncreaseCollateral
		// currentPos, _ := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), uint16(marketID))
		// errDb := models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), uint16(marketID), true, currentPos.HasDebt)
		// if errDb != nil {
		//     utils.Errorf("Failed to update margin active position for IncreaseCollateral event: %v", errDb)
		// }
		// For IncreaseCollateral: Set has_collateral = TRUE. Update last_activity_timestamp. Ensure is_active = TRUE.

		// TODO: Construct messagebus.TargetedMessage with ActualMessageType (e.g., MARGIN_ACCOUNT_UPDATE or specific like COLLATERAL_DEPOSITED)
		//       and ActualPayload (e.g., struct with user, marketID, asset, amount, new_balances_usd, new_debts_usd, new_collateral_ratio).
		//       Fetch updated account details (e.g., using sdk_wrappers.GetAccountDetails) to populate the payload fully.
		// err = messagebus.PublishToUserQueue(w.redisClient.Client, userAddress, "ACTUAL_MESSAGE_TYPE", actualPayload)
		// if err != nil { log.Printf("Error publishing to user queue: %v", err) }

	case "DecreaseCollateral":
		event, ok := marginContractABI.Events["DecreaseCollateral"]
		if !ok {
			utils.Errorf("DecreaseCollateral event definition not found in ABI")
			return
		}
		if len(logEntry.GetTopics()) < 4 { utils.Errorf("Not enough topics for DecreaseCollateral. Tx: %s", logEntry.GetTransactionHash()); return }
		user := ethcommon.BytesToAddress(logEntry.GetTopics()[1].Bytes())
		// marketID already parsed into eventMarketID_uint16
		asset := ethcommon.BytesToAddress(logEntry.GetTopics()[3].Bytes())
		var decodedDataDecrease struct { Amount *big.Int }
		if err := event.Inputs.NonIndexed().Unpack(&decodedDataDecrease, logEntry.GetData()); err != nil { utils.Errorf("Unpack DecreaseCollateral: %v. Tx: %s", err, logEntry.GetTransactionHash()); return }
		amount := decodedDataDecrease.Amount

		utils.Infof("DecreaseCollateral Event: User: %s, MarketID: %d, Asset: %s, Amount: %s",
			user.Hex(), eventMarketID_uint16, asset.Hex(), amount.String())
		userAddressHex = user.Hex()
		// TODO: DAO Usage Outline for DecreaseCollateral:
		// 1. Get the specific market details (baseAsset, quoteAsset) using marketID. (Requires models.MarketDao.FindMarketByMarketID(uint16(marketID)))
		// 2. Call sdk_wrappers.MarketBalanceOf for both base and quote assets for this userAddress & marketID. (Needs hydroSDK instance passed to handler or globally accessible)
		// 3. Determine newHasCollateral = (baseBalanceAfterEvent > 0 || quoteBalanceAfterEvent > 0).
		//    (Note: The event itself provides the *amount* decreased, not the final balance. Final balance must be fetched.)
		// 4. Fetch current hasDebt flag: currentPos, _ := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), uint16(marketID))
		// 5. Call models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), uint16(marketID), newHasCollateral, currentPos.HasDebt)
		//    if errDb != nil { utils.Errorf("Failed to update margin active position for DecreaseCollateral event: %v", errDb) }

	case "Borrow":
		event, ok := marginContractABI.Events["Borrow"]
		if !ok {
			utils.Errorf("Borrow event definition not found in ABI")
			return
		}
		if len(logEntry.GetTopics()) < 4 { utils.Errorf("Not enough topics for Borrow. Tx: %s", logEntry.GetTransactionHash()); return }
		user := ethcommon.BytesToAddress(logEntry.GetTopics()[1].Bytes())
		// marketID already parsed into eventMarketID_uint16
		asset := ethcommon.BytesToAddress(logEntry.GetTopics()[3].Bytes())
		var decodedDataBorrow struct { Amount *big.Int }
		if err := event.Inputs.NonIndexed().Unpack(&decodedDataBorrow, logEntry.GetData()); err != nil { utils.Errorf("Unpack Borrow: %v. Tx: %s", err, logEntry.GetTransactionHash()); return }
		amount := decodedDataBorrow.Amount

		utils.Infof("Borrow Event: User: %s, MarketID: %d, Asset: %s, Amount: %s",
			user.Hex(), eventMarketID_uint16, asset.Hex(), amount.String())
		userAddressHex = user.Hex()
		// TODO: DAO Usage Outline for Borrow:
		// currentPos, _ := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), uint16(marketID))
		// errDb := models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), uint16(marketID), currentPos.HasCollateral, true)
		// if errDb != nil {
		//     utils.Errorf("Failed to update margin active position for Borrow event: %v", errDb)
		// }
		// For Borrow: Set has_debt = TRUE. Update last_activity_timestamp. Ensure is_active = TRUE.

	case "Repay":
		event, ok := marginContractABI.Events["Repay"]
		if !ok {
			utils.Errorf("Repay event definition not found in ABI")
			return
		}
		if len(logEntry.GetTopics()) < 4 { utils.Errorf("Not enough topics for Repay. Tx: %s", logEntry.GetTransactionHash()); return }
		user := ethcommon.BytesToAddress(logEntry.GetTopics()[1].Bytes())
		// marketID already parsed into eventMarketID_uint16
		asset := ethcommon.BytesToAddress(logEntry.GetTopics()[3].Bytes())
		var decodedDataRepay struct { Amount *big.Int }
		if err := event.Inputs.NonIndexed().Unpack(&decodedDataRepay, logEntry.GetData()); err != nil { utils.Errorf("Unpack Repay: %v. Tx: %s", err, logEntry.GetTransactionHash()); return }
		amount := decodedDataRepay.Amount

		utils.Infof("Repay Event: User: %s, MarketID: %d, Asset: %s, Amount: %s",
			user.Hex(), eventMarketID_uint16, asset.Hex(), amount.String())
		userAddressHex = user.Hex()
		// TODO: DAO Usage Outline for Repay:
		// 1. Get the specific market details (baseAsset, quoteAsset) using marketID.
		// 2. Call sdk_wrappers.GetAmountBorrowed for both base and quote assets for this userAddress & marketID. (Needs hydroSDK instance)
		// 3. Determine newHasDebt = (baseDebtAfterEvent > 0 || quoteDebtAfterEvent > 0).
		//    (Note: The event itself provides the *amount* repaid, not the final debt. Final debt must be fetched.)
		// 4. Fetch current hasCollateral flag: currentPos, _ := models.MarginActivePositionDaoSql.GetOrCreate(user.Hex(), uint16(marketID))
		// 5. Call models.MarginActivePositionDaoSql.UpdateActivity(user.Hex(), uint16(marketID), currentPos.HasCollateral, newHasDebt)
		//    if errDb != nil { utils.Errorf("Failed to update margin active position for Repay event: %v", errDb) }

	default:
		utils.Debugf("Unhandled Margin Contract event type: %s", logEntry.GetEventName())
		return
	}

	// Publish TRIGGER_MARGIN_ACCOUNT_REFRESH message for all relevant events
	if userAddressHex != "" {
		eventMarketID_uint16 := uint16(new(big.Int).SetBytes(logEntry.GetTopics()[2].Bytes()).Uint64()) // Assuming topic[2] is always marketID for these events
		refreshPayload := map[string]interface{}{"marketID": eventMarketID_uint16, "reason": strings.ToLower(logEntry.GetEventName())}
		err := messagebus.PublishToUserQueue(h.redisClient.Client, userAddressHex, "TRIGGER_MARGIN_ACCOUNT_REFRESH", refreshPayload)
		if err != nil {
			utils.Errorf("Error publishing TRIGGER_MARGIN_ACCOUNT_REFRESH to user queue for user %s, event %s: %v", userAddressHex, logEntry.GetEventName(), err)
		} else {
			utils.Infof("Published TRIGGER_MARGIN_ACCOUNT_REFRESH for user %s, event %s", userAddressHex, logEntry.GetEventName())
		}
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

	//approve event should not process with engine, so update and return
	if launchLog.ItemType == "hydroApprove" {
		launchLog.Status = status
		err := models.LaunchLogDao.UpdateLaunchLog(launchLog)
		if err != nil {
			panic(err)
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
		Timestamp: txAndReceipt.TimeStamp, //todo
	}

	bts, _ := json.Marshal(event)

	err := handler.eventQueue.Push(bts)
	if err != nil {
		utils.Errorf("Push event into Queue Error: %v", err)
	}

	handler.kvStore.Set(common.HYDRO_WATCHER_BLOCK_NUMBER_CACHE_KEY, strconv.FormatUint(tx.GetBlockNumber(), 10), 0)
}

func main() {
	ctx, stop := context.WithCancel(context.Background())
	go cli.WaitExitSignal(stop)

	// Init Database Client
	models.Connect(os.Getenv("HSK_DATABASE_URL"))

	// Init Redis client
	redisClient := connection.NewRedisClient(os.Getenv("HSK_REDIS_URL"))

	// init Key/Value Store
	kvStore, err := common.InitKVStore(&common.RedisKVStoreConfig{
		Ctx:    ctx,
		Client: redisClient, // Corrected: use redisClient
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to init KVStore: %v", err))
	}

	queue, err := common.InitQueue(&common.RedisQueueConfig{
		Name:   common.HYDRO_ENGINE_EVENTS_QUEUE_KEY,
		Client: redisClient, // Corrected: use redisClient
		Ctx:    ctx,
	})
	if err != nil {
		panic(err)
	}

	// only interested in tx send by launcher
	filter := func(tx sdk.Transaction) bool {
		launchLog := models.LaunchLogDao.FindByHash(tx.GetHash())

		if launchLog == nil {
			utils.Debugf("Skip useless transaction %s", tx.GetHash())
			return false
		} else {
			return true
		}
	}

	dbTxHandler := DBTransactionHandler{
		eventQueue: queue,
		kvStore:    kvStore,
	}

	p := plugin.NewTxReceiptPluginWithFilter(dbTxHandler.TxHandlerFunc, filter)

	api := os.Getenv("HSK_BLOCKCHAIN_RPC_URL")
	w := nights_watch.NewHttpBasedEthWatcher(ctx, api)
	w.RegisterTxReceiptPlugin(txReceiptPlugin)

	// Initialize and Register Margin Contract Event Handler
	var errAbi error
	marginContractABI, errAbi = abi.JSON(strings.NewReader(MarginContractABIJsonString))
	if errAbi != nil {
		panic(fmt.Sprintf("Failed to parse MarginContractABIJsonString: %v", errAbi))
	}

	marginEventHandler := &MarginContractEventHandler{
		redisClient: redisClient,
	}

	// TODO: IMPORTANT - Replace this with the actual Margin Contract address from environment variable
	// HSK_MARGIN_CONTRACT_ADDRESS should be set in the environment.
	marginContractAddressHex := os.Getenv("HSK_MARGIN_CONTRACT_ADDRESS")
	if marginContractAddressHex == "" {
		utils.Warningf("HSK_MARGIN_CONTRACT_ADDRESS is not set. Margin event watcher will not be effective.")
		// Decide if this should be a panic or just a warning. For now, warning.
		// panic("HSK_MARGIN_CONTRACT_ADDRESS environment variable is required for watcher")
	}

	eventPlugin := plugin.NewContractEventPluginWithFilter(marginEventHandler, marginContractAddressHex, &marginContractABI, nil) // No specific event name filter, listen to all from this contract
	w.RegisterContractEventPlugin(eventPlugin)

	syncedBlockInCache, err := kvStore.Get(common.HYDRO_WATCHER_BLOCK_NUMBER_CACHE_KEY)
	if err != nil && err != common.KVStoreEmpty {
		panic(err)
	}

	var startFromBlock uint64
	if b, err := strconv.Atoi(syncedBlockInCache); err == nil {
		startFromBlock = uint64(b) + 1
	} else {
		startFromBlock = 0
	}

	go utils.StartMetrics()
	errRun := w.RunTillExitFromBlock(startFromBlock) // Renamed err to errRun to avoid conflict with errAbi
	if errRun != nil { // Corrected to check errRun
		utils.Infof("Watcher Exit with err: %s", errRun) // Corrected to log errRun
	} else {
		utils.Infof("Watcher Exit")
	}
}
