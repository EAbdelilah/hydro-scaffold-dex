package sdk_wrappers

import (
	"fmt"
	"math/big"
	"strings"

	// "github.com/HydroProtocol/hydro-scaffold-dex/backend/models" // Not directly used in this file but might be for more complex wrappers
	"context" // Added context
	"fmt"
	"math/big"
	"strings"

	// "github.com/HydroProtocol/hydro-scaffold-dex/backend/models" // Not directly used in this file but might be for more complex wrappers
	hydroSDKCommon "github.com/HydroProtocol/hydro-sdk-backend/common" // For types.GethCallMsg
	"github.com/HydroProtocol/hydro-sdk-backend/sdk"                   // Alias to avoid conflict if HydroSDK also has "sdk"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk/ethereum"         // For type assertion to EthereumHydro
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	goEthereumCommon "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
)

var hydroABI abi.ABI
var HydroContractAddress goEthereumCommon.Address

// InitHydroWrappers parses the Hydro ABI and sets the contract address.
// This should be called once during application startup.
func InitHydroWrappers(hydroContractAddressHex string) error {
	var err error
	hydroABI, err = abi.JSON(strings.NewReader(HydroContractABIJsonString))
	if err != nil {
		return fmt.Errorf("failed to parse Hydro ABI: %v", err)
	}

	if !goEthereumCommon.IsHexAddress(hydroContractAddressHex) {
		return fmt.Errorf("invalid hydro contract address from HSK_HYBRID_EXCHANGE_ADDRESS: %s", hydroContractAddressHex)
	}
	HydroContractAddress = goEthereumCommon.HexToAddress(hydroContractAddressHex)
	utils.Dump("Hydro Wrappers Initialized. Contract Address:", HydroContractAddress.Hex())
	return nil
}

// SDKAccountDetails mirrors the structure returned by the contract's getAccountDetails function.
type SDKAccountDetails struct {
	Liquidatable        bool
	Status              uint8 // 0 for Normal, 1 for Liquid (adjust based on actual contract enum)
	DebtsTotalUSDValue  *big.Int
	AssetsTotalUSDValue *big.Int
}

// GetAccountDetails calls the Hydro contract's getAccountDetails function.
func GetAccountDetails(hydro sdk.Hydro, userAddress goEthereumCommon.Address, marketID uint16) (*SDKAccountDetails, error) {
	// Ensure ABI is initialized
	if len(hydroABI.Methods) == 0 {
		return nil, fmt.Errorf("Hydro ABI not initialized in sdk_wrappers")
	}
	if HydroContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("HydroContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getAccountDetails"
	method, ok := hydroABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Hydro ABI", methodName)
	}

	// Prepare arguments for packing
	argsToPack := []interface{}{
		userAddress,
		marketID,
	}

	packedInput, err := hydroABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s: %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error

	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &HydroContractAddress, Data: packedInput} // Corrected type
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil) // nil for latest block
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: '%s' raw resultBytes: %x", methodName, resultBytes))

	var details SDKAccountDetails
	// Unpack the results directly into the struct.
	err = method.Outputs.UnpackIntoInterface(&details, resultBytes)
	if err != nil {
		// For more detailed error diagnosis if direct struct unpacking fails
		var outputInterfaces []interface{}
		// Attempt to unpack into a generic slice to see what was returned
		// Note: Unpacking into []interface{} might require knowing the number and types of outputs.
		// A simpler approach for raw output is just to log resultBytes as hex.
		// If the ABI specifies names for output arguments, UnpackIntoMap might also be useful for debugging.
		// mapOutput := make(map[string]interface{})
		// errMap := method.Outputs.UnpackIntoMap(mapOutput, resultBytes)
		// if errMap == nil {
		// 	 utils.Errorf("SDK_WRAPPER_ERROR: Successfully unpacked %s into map but not struct. Map Output: %v. Raw: %x", methodName, mapOutput, resultBytes)
		// } else {
		     utils.Errorf("SDK_WRAPPER_ERROR: Failed to unpack output for %s into SDKAccountDetails struct: %v. Also failed to unpack into map (err: %v). Raw: %x", methodName, err, "N/A for direct struct unpack", resultBytes)
		// }
		return nil, fmt.Errorf("failed to unpack output for %s: %v. Raw data: %x", methodName, err, resultBytes)
	}

	return &details, nil
}

// GetMarketTransferableAmount calls the Hydro contract's getMarketTransferableAmount function.
func GetMarketTransferableAmount(hydro sdk.Hydro, marketID uint16, assetAddress goEthereumCommon.Address, userAddress goEthereumCommon.Address) (*big.Int, error) {
	// Ensure ABI is initialized
	if len(hydroABI.Methods) == 0 {
		return nil, fmt.Errorf("Hydro ABI not initialized in sdk_wrappers")
	}
	if HydroContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("HydroContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getMarketTransferableAmount"
	method, ok := hydroABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Hydro ABI", methodName)
	}

	// Prepare arguments for packing - ensure order matches ABI definition
	argsToPack := []interface{}{
		marketID,
		assetAddress,
		userAddress,
	}

	packedInput, err := hydroABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s: %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &HydroContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: '%s' raw resultBytes: %x", methodName, resultBytes))

	results, err := method.Outputs.Unpack(resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s: %v. Raw: %x", methodName, err, resultBytes)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no output returned from %s, expected 1 (amount)", methodName)
	}

	amount, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("output from %s is not *big.Int, type is %T. Value: %v", methodName, results[0], results[0])
	}

	return amount, nil
}

// SDKActionType represents the type of action in a batch.
type SDKActionType uint8

const (
	SDKActionTypeDeposit            SDKActionType = 0 // Matches conceptual BatchActions.ActionType enum
	SDKActionTypeWithdraw           SDKActionType = 1
	SDKActionTypeTransfer           SDKActionType = 2
	SDKActionTypeBorrow             SDKActionType = 3
	SDKActionTypeRepay              SDKActionType = 4
	SDKActionTypeLiquidate          SDKActionType = 5
	SDKActionTypeAuctionPlaceBid    SDKActionType = 6
	SDKActionTypeAuctionClaim       SDKActionType = 7
)

// SDKBalanceCategory represents the category of a balance.
type SDKBalanceCategory uint8

const (
	SDKBalanceCategoryCommon            SDKBalanceCategory = 0 // Matches conceptual Types.BalanceCategory enum
	SDKBalanceCategoryCollateralAccount SDKBalanceCategory = 1
)

// SDKBalancePath mirrors the conceptual Types.BalancePath struct used in contract calls.
type SDKBalancePath struct {
	Category SDKBalanceCategory
	MarketID uint16
	User     goEthereumCommon.Address
}

// SDKBatchAction mirrors the conceptual BatchActions.Action struct.
type SDKBatchAction struct {
	ActionType    SDKActionType
	EncodedParams []byte
}

// EncodeTransferParamsForBatch ABI-encodes parameters for a Transfer action within a batch.
// Solidity signature for abi.decode: (address asset, Types.BalancePath memory fromBalancePath, Types.BalancePath memory toPath, uint256 amount)
// Types.BalancePath: (BalanceCategory category (uint8), uint16 marketID, address user)
func EncodeTransferParamsForBatch(
	assetAddress goEthereumCommon.Address,
	fromPath SDKBalancePath,
	toPath SDKBalancePath,
	amount *big.Int,
) ([]byte, error) {

	// Define argument types for abi.Arguments.Pack
	// Matched against (address, (uint8,uint16,address), (uint8,uint16,address), uint256)
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	uint16Type, _ := abi.NewType("uint16", "", nil)
	uint8Type, _ := abi.NewType("uint8", "", nil)

	balancePathComponents := []abi.ArgumentMarshaling{
		{Name: "category", Type: "uint8", InternalType: "uint8"}, // Ensure internal type matches if needed
		{Name: "marketID", Type: "uint16", InternalType: "uint16"},
		{Name: "user", Type: "address", InternalType: "address"},
	}
	balancePathTupleType, err := abi.NewType("tuple", "Types.BalancePath", balancePathComponents)
	if err != nil {
		return nil, fmt.Errorf("failed to create BalancePath tuple type for ABI packing: %v", err)
	}

	arguments := abi.Arguments{
		{Type: addressType, Name: "asset"},
		{Type: balancePathTupleType, Name: "fromBalancePath"},
		{Type: balancePathTupleType, Name: "toBalancePath"},
		{Type: uint256Type, Name: "amount"},
	}

	// The SDKBalancePath struct fields are: Category (SDKBalanceCategory which is uint8), MarketID (uint16), User (common.Address)
	// These should be compatible with direct packing if the Go struct fields are exported and match this order.
	packedBytes, err := arguments.Pack(
		assetAddress,
		fromPath, // Pass the Go struct directly
		toPath,   // Pass the Go struct directly
		amount,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to pack transfer params for batch: %v. Asset: %s, From: %+v, To: %+v, Amount: %s",
			err, assetAddress.Hex(), fromPath, toPath, amount.String())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Encoded Transfer Params for Batch (asset: %s, amount: %s): %x", assetAddress.Hex(), amount.String(), packedBytes))
	return packedBytes, nil
}

// ExecuteBatchActions calls the Hydro contract's batch function.
func ExecuteBatchActions(hydro sdk.Hydro, userAddress goEthereumCommon.Address, actions []SDKBatchAction) (goEthereumCommon.Hash, error) {
	// Ensure ABI is initialized
	if len(hydroABI.Methods) == 0 {
		return goEthereumCommon.Hash{}, fmt.Errorf("Hydro ABI not initialized in sdk_wrappers")
	}
	if HydroContractAddress == (goEthereumCommon.Address{}) {
		return goEthereumCommon.Hash{}, fmt.Errorf("HydroContractAddress not initialized in sdk_wrappers")
	}

	methodName := "batch"
	_, ok := hydroABI.Methods[methodName] // Check if method exists
	if !ok {
		return goEthereumCommon.Hash{}, fmt.Errorf("method %s not found in Hydro ABI", methodName)
	}

	// The `actions` parameter is a slice of SDKBatchAction structs.
	// SDKBatchAction struct: { ActionType SDKActionType (uint8), EncodedParams []byte }
	// Solidity equivalent for the Action struct in BatchActions.sol: struct Action { ActionType actionType; bytes encodedParams; }
	// The ABI packer should handle a slice of these Go structs as a dynamic array of tuples (uint8, bytes).
	packedInput, err := hydroABI.Pack(methodName, actions)
	if err != nil {
		// Provide more context on error, e.g. inspect `actions` content if complex
		return goEthereumCommon.Hash{}, fmt.Errorf("failed to pack input for %s with %d actions: %v. Actions: %+v", methodName, len(actions), err, actions)
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Call Hydro contract '%s' for User: %s with %d actions. PackedInput: %x", methodName, userAddress.Hex(), len(actions), packedInput))

	var txHash goEthereumCommon.Hash
	var txErr error

	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth {
		// TODO: CRITICAL - Implement actual transaction sending using hydroEth.
		// This requires understanding how hydro-sdk-backend handles:
		// 1. Transaction options (gas price, gas limit, nonce).
		// 2. Signing (if done client-side by the SDK using a local private key or via hardware wallet).
		// 3. Broadcasting the transaction.
		// Example conceptual steps (actual hydro SDK methods may vary significantly):
		// opts, err := hydroEth.NewTransactionOpts(context.Background(), userAddress, nil, nil) // gasPrice, gasLimit might be nil for auto
		// if err != nil { return common.Hash{}, fmt.Errorf("failed to create transaction opts: %v", err) }
		//
		// transaction, err := hydroEth.SendContractTransaction(opts, HydroContractAddress, packedInput)
		// if err != nil { txErr = err } else { txHash = transaction.Hash() }
		//
		// OR, if it's more manual:
		// rawTx, errBuild := hydroEth.BuildRawTransaction(userAddress, HydroContractAddress, packedInput, gasLimit, gasPrice, nonce, value)
		// signedTx, errSign := hydroEth.SignTransaction(rawTx)
		// txHash, txErr = hydroEth.SendRawTransaction(signedTx)

		txErr = fmt.Errorf("actual SendTransaction via SDK for '%s' not implemented in wrapper yet", methodName)
		// txHash = goEthereumCommon.HexToHash("0xSIMULATED_BATCH_TX_HASH_FROM_EXECUTE_" + userAddress.Hex()) // Keep error for now

		utils.Warning("TODO: ExecuteBatchActions - Actual transaction submission logic is a placeholder.")
	} else {
		txErr = fmt.Errorf("hydro SDK object cannot be asserted to *ethereum.EthereumHydro to send transaction for %s", methodName)
	}

	if txErr != nil {
		return goEthereumCommon.Hash{}, fmt.Errorf("transaction for %s failed: %v", methodName, txErr)
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: '%s' conceptual txHash: %s", methodName, txHash.Hex()))
	return txHash, nil
}

// Helper to convert string marketID to uint16, with error handling
func MarketIDToUint16(marketIDStr string) (uint16, error) {
	// TODO: Implement robust conversion, possibly involving a lookup if marketIDStr is not a direct number
	// For now, assuming it's a simple string representation of uint16 if used directly.
	// This is a placeholder. A real system might have a map or DB lookup.
	if marketIDStr == "ETH-DAI" || marketIDStr == "1" { // Example
		return 1, nil
	}
	// u, err := strconv.ParseUint(marketIDStr, 10, 16)
	// if err != nil {
	// 	return 0, fmt.Errorf("invalid marketID string for uint16 conversion: %s", marketIDStr)
	// }
	// return uint16(u), nil
	utils.Dump(fmt.Sprintf("Warning: MarketIDToUint16 using placeholder logic for MarketID: %s", marketIDStr))
	return 1, nil // Default placeholder
}

// --- Loan Service Wrappers ---

// SDKInterestRates mirrors the structure for interest rates.
type SDKInterestRates struct {
	BorrowInterestRate *big.Int // Per block or per second, needs clarification from contract
	SupplyInterestRate *big.Int // Per block or per second
}

// GetInterestRates calls the Hydro contract's getInterestRates function for a specific asset.
// extraBorrowAmount is optional, used by some contracts to calculate rates if borrowing more.
func GetInterestRates(hydro sdk.Hydro, assetAddress goEthereumCommon.Address, extraBorrowAmount *big.Int) (*SDKInterestRates, error) {
	// Ensure ABI is initialized
	if len(hydroABI.Methods) == 0 {
		return nil, fmt.Errorf("Hydro ABI not initialized in sdk_wrappers")
	}
	if HydroContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("HydroContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getInterestRates" // Assuming this is the contract method name
	method, ok := hydroABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Hydro ABI", methodName)
	}

	// Prepare arguments for packing
	argsToPack := []interface{}{
		assetAddress,
		extraBorrowAmount,
	}

	packedInput, err := hydroABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s: %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &HydroContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: '%s' raw resultBytes: %x", methodName, resultBytes))

	var ratesOutput SDKInterestRates
	err = method.Outputs.UnpackIntoInterface(&ratesOutput, resultBytes)
	if err != nil {
		var rawOutput []interface{}
		debugErr := method.Outputs.UnpackIntoInterface(&rawOutput, resultBytes) // Try to unpack into a slice for debugging
		if debugErr == nil {
			utils.Errorf("SDK_WRAPPER_ERROR: Unpacked %s into []interface{}: %v. Check SDKInterestRates struct definition. Raw: %x", methodName, rawOutput, resultBytes)
		}
		return nil, fmt.Errorf("failed to unpack output for %s into SDKInterestRates struct: %v. Raw: %x", methodName, err, resultBytes)
	}

	return &ratesOutput, nil
}

// GetAmountBorrowed conceptually calls the Hydro contract to get the amount of a specific asset borrowed by a user in a market.
func GetAmountBorrowed(hydro sdk.Hydro, userAddress goEthereumCommon.Address, marketID uint16, assetAddress goEthereumCommon.Address) (*big.Int, error) {
	// Ensure ABI is initialized
	if len(hydroABI.Methods) == 0 {
		return nil, fmt.Errorf("Hydro ABI not initialized in sdk_wrappers")
	}
	if HydroContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("HydroContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getAmountBorrowed"
	method, ok := hydroABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Hydro ABI", methodName)
	}

	// Prepare arguments for packing - ENSURE ORDER MATCHES ABI: assetAddress, userAddress, marketID
	argsToPack := []interface{}{
		assetAddress,
		userAddress,
		marketID,
	}

	packedInput, err := hydroABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s: %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &HydroContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: '%s' raw resultBytes: %x", methodName, resultBytes))

	results, err := method.Outputs.Unpack(resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s: %v. Raw: %x", methodName, err, resultBytes)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no output returned from %s, expected 1 (amount)", methodName)
	}

	amount, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("output from %s is not *big.Int, type is %T. Value: %v", methodName, results[0], results[0])
	}

	return amount, nil
}

// EncodeBorrowParamsForBatch ABI-encodes parameters for a Borrow action.
// Solidity: (uint16 marketID, address asset, uint256 amount)
func EncodeBorrowParamsForBatch(
	marketID uint16,
	assetAddress goEthereumCommon.Address,
	amount *big.Int,
) ([]byte, error) {
	uint16Type, _ := abi.NewType("uint16", "", nil)
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)

	arguments := abi.Arguments{
		{Type: uint16Type, Name: "marketID"},
		{Type: addressType, Name: "asset"},
		{Type: uint256Type, Name: "amount"},
	}

	packedBytes, err := arguments.Pack(
		marketID,
		assetAddress,
		amount,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to pack borrow params for batch: %v. marketID: %d, asset: %s, amount: %s",
			err, marketID, assetAddress.Hex(), amount.String())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Encoded Borrow Params for Batch (marketID: %d, asset: %s, amount: %s): %x", marketID, assetAddress.Hex(), amount.String(), packedBytes))
	return packedBytes, nil
}

// EncodeRepayParamsForBatch ABI-encodes parameters for a Repay action.
// Solidity: (uint16 marketID, address asset, uint256 amount)
func EncodeRepayParamsForBatch(
	marketID uint16,
	assetAddress goEthereumCommon.Address,
	amount *big.Int,
) ([]byte, error) {
	uint16Type, _ := abi.NewType("uint16", "", nil)
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)

	arguments := abi.Arguments{
		{Type: uint16Type, Name: "marketID"},
		{Type: addressType, Name: "asset"},
		{Type: uint256Type, Name: "amount"},
	}

	packedBytes, err := arguments.Pack(
		marketID,
		assetAddress,
		amount,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to pack repay params for batch: %v. marketID: %d, asset: %s, amount: %s",
			err, marketID, assetAddress.Hex(), amount.String())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Encoded Repay Params for Batch (marketID: %d, asset: %s, amount: %s): %x", marketID, assetAddress.Hex(), amount.String(), packedBytes))
	return packedBytes, nil
}

// --- Order Service Wrappers ---

// GenerateMarginOrderDataHex manually constructs the bytes32 data field for Hydro orders,
// accommodating margin-specific parameters like balanceCategory and orderDataMarketID.
// This is needed if the existing SDK's GenerateOrderData doesn't support these directly.
func GenerateMarginOrderDataHex(
	version int64, // e.g., 2
	expiredAtSec int64,
	salt int64,
	asMakerFeeRate decimal.Decimal, // e.g., 0.001 for 0.1%
	asTakerFeeRate decimal.Decimal, // e.g., 0.002 for 0.2%
	makerRebateRate decimal.Decimal, // e.g., 0 for no rebate
	isSell bool,
	isMarketOrder bool,
	balanceCategory SDKBalanceCategory, // From our defined enum
	orderDataMarketID uint16, // The marketID for the collateral account, or 0 if common
	isMakerOnly bool,
) (string, error) {
	utils.Dump("SDK_WRAPPER_INFO: Manually pack bytes32 order data for margin order.", version, expiredAtSec, salt, asMakerFeeRate, asTakerFeeRate, makerRebateRate, isSell, isMarketOrder, balanceCategory, orderDataMarketID, isMakerOnly)

	// TODO: CRITICAL - Implement actual bit-packing logic here according to Hydro's Types.sol OrderParam.data specification.
	// The structure is approximately:
	// version (1 byte)                 [0]
	// side (1 byte: 0 for buy, 1 for sell) [1]
	// isMarketOrder (1 byte: 0 for limit, 1 for market) [2]
	// expiredAt (5 bytes: uint40 seconds) [3:7]
	// asMakerFeeRate (2 bytes: uint16, rate * 100000) [8:9] (e.g. 0.1% = 100)
	// asTakerFeeRate (2 bytes: uint16, rate * 100000) [10:11]
	// makerRebateRate (2 bytes: uint16, rate * 100000, not 100 as previously mis-commented) [12:13]
	// salt (8 bytes: uint64) [14:21]
	// isMakerOnly (1 byte) [22]
	// balanceCategory (1 byte: uint8) [23]
	// marketID (2 bytes: uint16) [24:25] (This is orderDataMarketID)
	// reserved (6 bytes) [26:31]

	// For now, returning an error is safer as this is critical and not implemented.
	// If a placeholder hex is needed for unit testing unrelated parts, ensure it's clearly marked.
	// Example: return "0x02000000000000006400c80000000000000000000100010000000000000000", nil
	return "", fmt.Errorf("GenerateMarginOrderDataHex bit-packing is not implemented and is critical for margin orders")
}
