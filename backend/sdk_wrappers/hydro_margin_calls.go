package sdk_wrappers

import (
	"context" // Added context
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	hydroSDKCommon "github.com/HydroProtocol/hydro-sdk-backend/common" // For types.GethCallMsg
	"github.com/HydroProtocol/hydro-sdk-backend/sdk"                   // Alias to avoid conflict if HydroSDK also has "sdk"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk/ethereum"         // For type assertion to EthereumHydro
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	goEthereumCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/shopspring/decimal"
)

var hydroABI abi.ABI
var HydroContractAddress goEthereumCommon.Address
var marginContractABI abi.ABI
var MarginContractAddress goEthereumCommon.Address
var genericPriceOracleABI abi.ABI // Added for GenericPriceOracle

// InitHydroWrappers parses ABIs and sets contract addresses.
// hydroContractAddressHex is for the existing exchange contract.
// marginContractAddressHex is for the new Margin Contract.
func InitHydroWrappers(hydroContractAddressHex string, marginContractAddressHex string) error {
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

	// Initialize New Margin Contract ABI and Address
	marginContractABI, err = abi.JSON(strings.NewReader(MarginContractABIJsonString)) // MarginContractABIJsonString is from hydro_contract_abis.go
	if err != nil {
		return fmt.Errorf("failed to parse Margin Contract ABI: %v", err)
	}

	if !goEthereumCommon.IsHexAddress(marginContractAddressHex) {
		return fmt.Errorf("invalid margin contract address: %s", marginContractAddressHex)
	}
	MarginContractAddress = goEthereumCommon.HexToAddress(marginContractAddressHex)
	utils.Dump("Margin Contract Wrappers Initialized. Address:", MarginContractAddress.Hex())

	// TODO: Consider if genericPriceOracleABI initialization is still needed or if it's part of a different setup.
	// For now, keeping it as it was, assuming it's handled elsewhere if this function was its only initializer.
	// If it was initialized here, it needs to be decided if it's still relevant.
	// Example:
	// genericPriceOracleABI, err = abi.JSON(strings.NewReader(GenericPriceOracleABIJsonString))
	// if err != nil {
	// 	return fmt.Errorf("failed to parse GenericPriceOracle ABI: %v", err)
	// }
	// utils.Dump("GenericPriceOracle ABI Initialized.")

	// Initialize Generic Price Oracle ABI
	genericPriceOracleABI, err = abi.JSON(strings.NewReader(GenericPriceOracleABIJsonString))
	if err != nil {
		return fmt.Errorf("failed to parse GenericPriceOracle ABI: %v", err)
	}
	utils.Dump("GenericPriceOracle ABI Initialized.")

	return nil
}

// SDKAccountDetails mirrors the structure returned by the contract's getAccountDetails function.
// Note: The ABI output for getAccountDetails is a tuple named "details".
// The struct fields must match the component names in that tuple for direct UnpackIntoInterface.
type SDKAccountDetails struct {
	Liquidatable        bool     `abi:"liquidatable"`
	Status              uint8    `abi:"status"`
	DebtsTotalUSDValue  *big.Int `abi:"debtsTotalUSDValue"`
	AssetsTotalUSDValue *big.Int `abi:"balancesTotalUSDValue"` // ABI name is balancesTotalUSDValue
}

// GetAccountDetails calls the Hydro contract's getAccountDetails function.
func GetAccountDetails(hydro sdk.Hydro, userAddress goEthereumCommon.Address, marketID uint16) (*SDKAccountDetails, error) {
	// Ensure ABI is initialized
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getAccountDetails"
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	// Prepare arguments for packing
	argsToPack := []interface{}{
		userAddress,
		marketID,
	}

	packedInput, err := marginContractABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s (Margin Contract): %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error

	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil) // nil for latest block
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s (Margin Contract)", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("Margin Contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("Margin Contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' raw resultBytes: %x", methodName, resultBytes))

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
		// 	 utils.Errorf("SDK_WRAPPER_ERROR: Successfully unpacked %s (Margin Contract) into map but not struct. Map Output: %v. Raw: %x", methodName, mapOutput, resultBytes)
		// } else {
		     utils.Errorf("SDK_WRAPPER_ERROR: Failed to unpack output for %s (Margin Contract) into SDKAccountDetails struct: %v. Also failed to unpack into map (err: %v). Raw: %x", methodName, err, "N/A for direct struct unpack", resultBytes)
		// }
		return nil, fmt.Errorf("failed to unpack output for %s (Margin Contract): %v. Raw data: %x", methodName, err, resultBytes)
	}

	return &details, nil
}

// GetMarketTransferableAmount calls the Margin contract's getMarketTransferableAmount function.
func GetMarketTransferableAmount(hydro sdk.Hydro, marketID uint16, assetAddress goEthereumCommon.Address, userAddress goEthereumCommon.Address) (*big.Int, error) {
	// Ensure ABI is initialized
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getMarketTransferableAmount"
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		// Since this method might not be in the placeholder MarginContractABIJsonString, we add a specific check.
		// If it's expected to be part of the *actual* Margin ABI, this error is valid.
		// If not, this function might need to be removed or adapted if it's not for the margin contract.
		return nil, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	// Prepare arguments for packing - ensure order matches ABI definition
	argsToPack := []interface{}{
		marketID,
		assetAddress,
		userAddress,
	}

	packedInput, err := marginContractABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s (Margin Contract): %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s (Margin Contract)", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("Margin Contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("Margin Contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' raw resultBytes: %x", methodName, resultBytes))

	results, err := method.Outputs.Unpack(resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s (Margin Contract): %v. Raw: %x", methodName, err, resultBytes)
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

// MarketBalanceOf calls the Margin contract's marketBalanceOf function.
// This function retrieves the balance of a specific asset for a user within a given market context.
func MarketBalanceOf(hydro sdk.Hydro, marketID uint16, assetAddress goEthereumCommon.Address, userAddress goEthereumCommon.Address) (*big.Int, error) {
	// Ensure ABI is initialized
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "marketBalanceOf"
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	// Prepare arguments for packing - ensure order matches ABI definition: marketID, asset, user
	argsToPack := []interface{}{
		marketID,
		assetAddress,
		userAddress,
	}

	packedInput, err := marginContractABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s (Margin Contract): %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s (Margin Contract)", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("Margin Contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("Margin Contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' raw resultBytes: %x", methodName, resultBytes))

	results, err := method.Outputs.Unpack(resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s (Margin Contract): %v. Raw: %x", methodName, err, resultBytes)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no output returned from %s (Margin Contract), expected 1 (balance)", methodName)
	}

	balance, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("output from %s (Margin Contract) is not *big.Int, type is %T. Value: %v", methodName, results[0], results[0])
	}

	return balance, nil
}

// SDKActionType represents the type of action in a batch.
type SDKActionType uint8

const (
	SDKActionTypeDeposit            SDKActionType = 0 // Matches conceptual BatchActions.ActionType enum
	SDKActionTypeWithdraw           SDKActionType = 1
	SDKActionTypeTransfer           SDKActionType = 2
	SDKActionTypeBorrow             SDKActionType = 3
	SDKActionTypeRepay              SDKActionType = 4
	SDKActionTypeSupply             SDKActionType = 5 // Matches BatchActions.ActionType.Supply
	SDKActionTypeUnsupply           SDKActionType = 6 // Matches BatchActions.ActionType.Unsupply
	// The following ActionTypes are not standard in BatchActions.sol and should not be used with the generic Hydro.batch() call.
	// If these operations (Liquidation, Auction Bidding/Claiming) are implemented, they likely require direct calls to specific smart contract functions, not batching through BatchActions.ActionType.
	// SDKActionTypeLiquidate          SDKActionType = 7 // Example: if it were a direct mapping (originally 5 before Supply/Unsupply)
	// SDKActionTypeAuctionPlaceBid    SDKActionType = 8 // Example (originally 6)
	// SDKActionTypeAuctionClaim       SDKActionType = 9 // Example (originally 7)
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

// ExecuteBatchActions calls the Margin contract's batch function.
func ExecuteBatchActions(hydro sdk.Hydro, userAddress goEthereumCommon.Address, actions []SDKBatchAction) (goEthereumCommon.Hash, error) {
	// Ensure ABI is initialized
	if len(marginContractABI.Methods) == 0 {
		return goEthereumCommon.Hash{}, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return goEthereumCommon.Hash{}, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "batch"
	_, ok := marginContractABI.Methods[methodName] // Check if method exists
	if !ok {
		return goEthereumCommon.Hash{}, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	// The `actions` parameter is a slice of SDKBatchAction structs.
	// SDKBatchAction struct: { ActionType SDKActionType (uint8), EncodedParams []byte }
	// Solidity equivalent for the Action struct in BatchActions.sol: struct Action { ActionType actionType; bytes encodedParams; }
	// The ABI packer should handle a slice of these Go structs as a dynamic array of tuples (uint8, bytes).
	packedInput, err := marginContractABI.Pack(methodName, actions)
	if err != nil {
		// Provide more context on error, e.g. inspect `actions` content if complex
		return goEthereumCommon.Hash{}, fmt.Errorf("failed to pack input for %s (Margin Contract) with %d actions: %v. Actions: %+v", methodName, len(actions), err, actions)
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Call Margin Contract '%s' for User: %s with %d actions. PackedInput: %x", methodName, userAddress.Hex(), len(actions), packedInput))

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
		// transaction, err := hydroEth.SendContractTransaction(opts, MarginContractAddress, packedInput) // IMPORTANT: Use MarginContractAddress
		// if err != nil { txErr = err } else { txHash = transaction.Hash() }
		//
		// OR, if it's more manual:
		// rawTx, errBuild := hydroEth.BuildRawTransaction(userAddress, MarginContractAddress, packedInput, gasLimit, gasPrice, nonce, value)  // IMPORTANT: Use MarginContractAddress
		// signedTx, errSign := hydroEth.SignTransaction(rawTx)
		// txHash, txErr = hydroEth.SendRawTransaction(signedTx)

		txErr = fmt.Errorf("actual SendTransaction via SDK for '%s' (Margin Contract) not implemented in wrapper yet", methodName)
		// txHash = goEthereumCommon.HexToHash("0xSIMULATED_BATCH_TX_HASH_FROM_EXECUTE_MARGIN_" + userAddress.Hex()) // Keep error for now

		utils.Warning("TODO: ExecuteBatchActions (Margin Contract) - Actual transaction submission logic is a placeholder.")
	} else {
		txErr = fmt.Errorf("hydro SDK object cannot be asserted to *ethereum.EthereumHydro to send transaction for %s (Margin Contract)", methodName)
	}

	if txErr != nil {
		return goEthereumCommon.Hash{}, fmt.Errorf("transaction for %s (Margin Contract) failed: %v", methodName, txErr)
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' conceptual txHash: %s", methodName, txHash.Hex()))
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

// GetInterestRates calls the Margin contract's getInterestRates function for a specific asset.
// extraBorrowAmount is optional, used by some contracts to calculate rates if borrowing more.
func GetInterestRates(hydro sdk.Hydro, assetAddress goEthereumCommon.Address, extraBorrowAmount *big.Int) (*SDKInterestRates, error) {
	// Ensure ABI is initialized
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getInterestRates" // Assuming this is the contract method name
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	// Prepare arguments for packing
	argsToPack := []interface{}{
		assetAddress,
		extraBorrowAmount,
	}

	packedInput, err := marginContractABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s (Margin Contract): %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s (Margin Contract)", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("Margin Contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("Margin Contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' raw resultBytes: %x", methodName, resultBytes))

	var ratesOutput SDKInterestRates
	err = method.Outputs.UnpackIntoInterface(&ratesOutput, resultBytes)
	if err != nil {
		var rawOutput []interface{}
		debugErr := method.Outputs.UnpackIntoInterface(&rawOutput, resultBytes) // Try to unpack into a slice for debugging
		if debugErr == nil {
			utils.Errorf("SDK_WRAPPER_ERROR: Unpacked %s (Margin Contract) into []interface{}: %v. Check SDKInterestRates struct definition. Raw: %x", methodName, rawOutput, resultBytes)
		}
		return nil, fmt.Errorf("failed to unpack output for %s (Margin Contract) into SDKInterestRates struct: %v. Raw: %x", methodName, err, resultBytes)
	}

	return &ratesOutput, nil
}

// GetAmountBorrowed conceptually calls the Margin contract to get the amount of a specific asset borrowed by a user in a market.
func GetAmountBorrowed(hydro sdk.Hydro, userAddress goEthereumCommon.Address, marketID uint16, assetAddress goEthereumCommon.Address) (*big.Int, error) {
	// Ensure ABI is initialized
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getAmountBorrowed"
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	// Prepare arguments for packing - ENSURE ORDER MATCHES ABI: assetAddress, userAddress, marketID
	argsToPack := []interface{}{
		assetAddress,
		userAddress,
		marketID,
	}

	packedInput, err := marginContractABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s (Margin Contract): %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient or generic call method for %s (Margin Contract)", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("Margin Contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("Margin Contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' raw resultBytes: %x", methodName, resultBytes))

	results, err := method.Outputs.Unpack(resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s (Margin Contract): %v. Raw: %x", methodName, err, resultBytes)
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

// --- Additional Margin Contract Specific Wrappers ---

// SDKAsset represents the structure returned by the Margin Contract's `getAsset` method.
type SDKAsset struct {
	LendingPoolToken goEthereumCommon.Address `abi:"lendingPoolToken"`
	PriceOracle      goEthereumCommon.Address `abi:"priceOracle"`
	InterestModel    goEthereumCommon.Address `abi:"interestModel"`
	// Add other fields from the ABI's Asset_Data struct if they are part of the tuple
}

// GetAsset calls the Margin contract's getAsset function.
// This function retrieves detailed information about a specific asset in the margin system.
// Note: The ABI method name is "getAsset".
func GetAsset(hydro sdk.Hydro, assetAddress goEthereumCommon.Address) (*SDKAsset, error) {
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getAsset" // ABI method name
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	argsToPack := []interface{}{assetAddress}
	packedInput, err := marginContractABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s (Margin Contract): %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient for %s (Margin Contract)", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("Margin Contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		// The getAsset method returns a tuple, so it should not return empty data.
		return nil, fmt.Errorf("Margin Contract call to %s returned no data but expected a tuple", methodName)
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' raw resultBytes: %x", methodName, resultBytes))

	var assetOutput SDKAsset
	// The output is a single tuple named "asset". Unpack directly into the struct.
	err = method.Outputs.UnpackIntoInterface(&assetOutput, resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s (Margin Contract) into SDKAsset: %v. Raw: %x", methodName, err, resultBytes)
	}

	return &assetOutput, nil
}


// GetOraclePrice calls the getPrice(address asset) method of a given price oracle contract.
func GetOraclePrice(hydro sdk.Hydro, oracleAddress goEthereumCommon.Address, assetAddress goEthereumCommon.Address) (*big.Int, error) {
	if len(genericPriceOracleABI.Methods) == 0 {
		return nil, fmt.Errorf("GenericPriceOracle ABI not initialized")
	}

	methodName := "getPrice"
	method, ok := genericPriceOracleABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in GenericPriceOracle ABI", methodName)
	}

	// Pack arguments for the getPrice(address asset) call
	packedArgs, err := method.Inputs.Pack(assetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to pack arguments for %s (Oracle: %s, Asset: %s): %v", methodName, oracleAddress.Hex(), assetAddress.Hex(), err)
	}

	// Construct call data: methodID + packedArgs
	// Method ID is the first 4 bytes of the Keccak256 hash of the function signature.
	// The abi.Method object contains this ID.
	data := append(method.ID, packedArgs...)


	var responseData []byte
	var callErr error

	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &oracleAddress, Data: data}
		responseData, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient for oracle call %s", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("oracle contract call to %s (Oracle: %s, Asset: %s) failed: %v", methodName, oracleAddress.Hex(), assetAddress.Hex(), callErr)
	}
	if len(responseData) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("oracle contract call to %s returned no data but expected %d outputs", methodName, method.Outputs.Length())
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Oracle '%s' (Oracle: %s, Asset: %s) raw responseData: %x", methodName, oracleAddress.Hex(), assetAddress.Hex(), responseData))

	results, err := method.Outputs.Unpack(responseData)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s (Oracle: %s, Asset: %s): %v. Raw: %x", methodName, oracleAddress.Hex(), assetAddress.Hex(), err, responseData)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no output returned from oracle %s, expected 1 (price)", methodName)
	}

	price, ok := results[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("output from oracle %s is not *big.Int, type is %T. Value: %v", methodName, results[0], results[0])
	}

	utils.Infof("Oracle price for asset %s from oracle %s: %s", assetAddress.Hex(), oracleAddress.Hex(), price.String())
	return price, nil
}


// SDKAuctionDetails mirrors the structure for auction details from the Margin contract.
// TODO: Define the actual fields based on the contract's getAuctionDetails output struct.
type SDKAuctionDetails struct {
	// Example fields - replace with actual ones:
	Auctioneer        goEthereumCommon.Address
	AssetToSell       goEthereumCommon.Address
	AmountToSell      *big.Int
	AssetToBuy        goEthereumCommon.Address
	CurrentBid        *big.Int
	CurrentBidder     goEthereumCommon.Address
	EndTime           *big.Int
	IsActive          bool
	LiquidationStatus uint8 // Enum for status like Ongoing, Claimable, Ended
}

// GetAuctionDetails calls the Margin contract's getAuctionDetails function.
// This retrieves details for a specific auction, typically identified by an auction ID or user and market.
func GetAuctionDetails(hydro sdk.Hydro, auctionID *big.Int) (*SDKAuctionDetails, error) { // Assuming auctionID is *big.Int
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("Margin Contract ABI not initialized in sdk_wrappers")
	}
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("MarginContractAddress not initialized in sdk_wrappers")
	}

	methodName := "getAuctionDetails"
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("method %s not found in Margin Contract ABI", methodName)
	}

	argsToPack := []interface{}{auctionID} // Adjust if auction ID is not the only param
	packedInput, err := marginContractABI.Pack(methodName, argsToPack...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack input for %s (Margin Contract): %v", methodName, err)
	}

	var resultBytes []byte
	var callErr error
	if hydroEth, okEth := hydro.(*ethereum.EthereumHydro); okEth && hydroEth.EthClient() != nil {
		ethCl := hydroEth.EthClient()
		callMsg := hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: packedInput}
		resultBytes, callErr = ethCl.CallContract(context.Background(), callMsg, nil)
	} else {
		return nil, fmt.Errorf("hydro SDK object does not provide a usable ethclient for %s (Margin Contract)", methodName)
	}

	if callErr != nil {
		return nil, fmt.Errorf("Margin Contract call to %s failed: %v", methodName, callErr)
	}
	if len(resultBytes) == 0 && method.Outputs.Length() > 0 {
		return nil, fmt.Errorf("Margin Contract call to %s returned no data", methodName)
	}

	utils.Dump(fmt.Sprintf("SDK_WRAPPER_DEBUG: Margin Contract '%s' raw resultBytes: %x", methodName, resultBytes))

	var auctionDetails SDKAuctionDetails
	err = method.Outputs.UnpackIntoInterface(&auctionDetails, resultBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack output for %s (Margin Contract) into SDKAuctionDetails: %v. Raw: %x", methodName, err, resultBytes)
	}

	return &auctionDetails, nil
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

	// Structure for bytes32 data field according to Types.sol OrderParam.data:
	// Field             | Bits | Bytes | Offset (Bytes) | Notes
	// ------------------|------|-------|----------------|-------------------------------------------------
	// version           | 8    | 1     | 0              |
	// side              | 8    | 1     | 1              | 0 for buy, 1 for sell
	// isMarketOrder     | 8    | 1     | 2              | 0 for limit, 1 for market
	// expiredAt         | 40   | 5     | 3              | uint40 seconds
	// asMakerFeeRate    | 16   | 2     | 8              | uint16, actual rate * 100000 (e.g., 0.1% = 100)
	// asTakerFeeRate    | 16   | 2     | 10             | uint16, actual rate * 100000
	// makerRebateRate   | 16   | 2     | 12             | uint16, actual rate * 100000
	// salt              | 64   | 8     | 14             | uint64
	// isMakerOnly       | 8    | 1     | 22             | 0 for false, 1 for true
	// balanceCategory   | 8    | 1     | 23             | uint8, 0 for Common, 1 for CollateralAccount
	// marketID          | 16   | 2     | 24             | uint16, orderDataMarketID (collateral market)
	// reserved          | 48   | 6     | 26             |
	// Total             | 256  | 32    |                |

	var orderDataBytes [32]byte

	// version (1 byte) [0]
	orderDataBytes[0] = byte(version)

	// side (1 byte: 0 for buy, 1 for sell) [1]
	if isSell {
		orderDataBytes[1] = 1
	} else {
		orderDataBytes[1] = 0
	}

	// isMarketOrder (1 byte: 0 for limit, 1 for market) [2]
	if isMarketOrder {
		orderDataBytes[2] = 1
	} else {
		orderDataBytes[2] = 0
	}

	// expiredAt (5 bytes, uint40 seconds) [3:7]
	// We need to take the lower 5 bytes of the int64.
	// binary.BigEndian.PutUint64 expects an 8-byte slice.
	var expiredAtBytes [8]byte
	binary.BigEndian.PutUint64(expiredAtBytes[:], uint64(expiredAtSec))
	copy(orderDataBytes[3:8], expiredAtBytes[3:8]) // Copy the lower 5 bytes (uint40)

	// asMakerFeeRate (2 bytes, uint16, rate * 100000) [8:9]
	makerFeeRateInt := asMakerFeeRate.Mul(decimal.NewFromInt(100000)).IntPart()
	binary.BigEndian.PutUint16(orderDataBytes[8:10], uint16(makerFeeRateInt))

	// asTakerFeeRate (2 bytes, uint16, rate * 100000) [10:11]
	takerFeeRateInt := asTakerFeeRate.Mul(decimal.NewFromInt(100000)).IntPart()
	binary.BigEndian.PutUint16(orderDataBytes[10:12], uint16(takerFeeRateInt))

	// makerRebateRate (2 bytes, uint16, rate * 100000) [12:13]
	// Solidity's OrderParam.getMakerRebateRateFromOrderData uses min(value, Consts.REBATE_RATE_BASE()) where REBATE_RATE_BASE is 100.
	// This means the packed uint16 value should represent the rebate as a percentage scaled by 1 (e.g., 1 for 1%, 50 for 50%).
	// If makerRebateRate (decimal.Decimal) is a fraction (e.g., 0.01 for 1%), scale by 100.
	makerRebateRateInt := makerRebateRate.Mul(decimal.NewFromInt(100)).IntPart()
	binary.BigEndian.PutUint16(orderDataBytes[12:14], uint16(makerRebateRateInt))

	// salt (8 bytes, uint64) [14:21]
	binary.BigEndian.PutUint64(orderDataBytes[14:22], uint64(salt))

	// isMakerOnly (1 byte) [22]
	if isMakerOnly {
		orderDataBytes[22] = 1
	} else {
		orderDataBytes[22] = 0
	}

	// balanceCategory (1 byte, uint8) [23]
	orderDataBytes[23] = byte(balanceCategory)

	// marketID (2 bytes, uint16) [24:25] (This is orderDataMarketID)
	binary.BigEndian.PutUint16(orderDataBytes[24:26], orderDataMarketID)

	// reserved (6 bytes) [26:31] - already zeros by default initialization of array

	return hexutil.Encode(orderDataBytes[:]), nil
}

// --- Market Parameter Wrappers ---

// MarketMarginParams holds margin-specific parameters for a market.
type MarketMarginParams struct {
	InitialMarginFraction     decimal.Decimal // e.g., 0.2 for 20%
	MaintenanceMarginFraction decimal.Decimal // e.g., 0.1 for 10%
	LiquidateRate             decimal.Decimal // e.g., 1.1 for 110%
	// Add other fields like borrowEnable, specific asset params if needed from contract's getMarket
}

// GetMarketMarginParameters fetches margin parameters for a given marketID.
// It first attempts to find specific getters for IMR and MMR in the ABI.
// If not found, it falls back to deriving them from the on-chain LiquidateRate.
func GetMarketMarginParameters(hydro sdk.Hydro, marketID uint16) (*MarketMarginParams, error) {
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("GetMarketMarginParameters: MarginContractAddress not initialized")
	}
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("GetMarketMarginParameters: Margin Contract ABI not initialized (ABI methods map is empty)")
	}

	params := &MarketMarginParams{}
	var err error
	var callErr error
	var hydroEth *ethereum.EthereumHydro
	var okEth bool

	hydroEth, okEth = hydro.(*ethereum.EthereumHydro)
	if !okEth || hydroEth.EthClient() == nil {
		return nil, fmt.Errorf("GetMarketMarginParameters: hydro SDK object does not provide a usable Ethereum client")
	}
	ethCl := hydroEth.EthClient()

	// 1. Fetch LiquidateRate from getMarket method (always needed)
	getMarketMethodName := "getMarket"
	getMarketMethod, ok := marginContractABI.Methods[getMarketMethodName]
	if !ok {
		return nil, fmt.Errorf("GetMarketMarginParameters: method '%s' not found in MarginContractABI. Ensure ABI string is complete.", getMarketMethodName)
	}
	marketArgsToPack := []interface{}{marketID}
	marketPackedInput, packErr := marginContractABI.Pack(getMarketMethodName, marketArgsToPack...)
	if packErr != nil {
		return nil, fmt.Errorf("GetMarketMarginParameters: failed to pack args for %s: %w", getMarketMethodName, packErr)
	}
	marketResultBytes, marketCallErr := ethCl.CallContract(context.Background(), hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: marketPackedInput}, nil)
	if marketCallErr != nil {
		return nil, fmt.Errorf("GetMarketMarginParameters: contract call to %s failed for marketID %d: %w", getMarketMethodName, marketID, marketCallErr)
	}
	if len(marketResultBytes) == 0 && getMarketMethod.Outputs.Length() > 0 {
		return nil, fmt.Errorf("GetMarketMarginParameters: contract call to %s returned no data for marketID %d", getMarketMethodName, marketID)
	}
	var marketDataTuple struct {
		BaseAsset            goEthereumCommon.Address `abi:"baseAsset"`
		QuoteAsset           goEthereumCommon.Address `abi:"quoteAsset"`
		LiquidateRate        *big.Int                 `abi:"liquidateRate"`
		WithdrawRate         *big.Int                 `abi:"withdrawRate"`
		AuctionRatioStart    *big.Int                 `abi:"auctionRatioStart"`
		AuctionRatioPerBlock *big.Int                 `abi:"auctionRatioPerBlock"`
		BorrowEnable         bool                     `abi:"borrowEnable"`
	}
	unpackErr := getMarketMethod.Outputs.UnpackIntoInterface(&marketDataTuple, marketResultBytes)
	if unpackErr != nil {
		return nil, fmt.Errorf("GetMarketMarginParameters: failed to unpack %s output for marketID %d: %w. Raw: %s", getMarketMethodName, marketID, unpackErr, hexutil.Encode(marketResultBytes))
	}

	if marketDataTuple.LiquidateRate != nil {
		params.LiquidateRate = decimal.NewFromBigInt(marketDataTuple.LiquidateRate, -18)
	} else {
		params.LiquidateRate = decimal.Zero
		utils.Warningf("GetMarketMarginParameters: LiquidateRate from contract (getMarket) was nil for marketID %d. Defaulting to zero.", marketID)
	}

	// 2. Attempt to fetch InitialMarginFraction directly from chain.
	// Section for fetching InitialMarginFraction directly from chain commented out as 'getInitialMarginFraction(marketID)'
	// is not found in the provided MarginContractABI or Hydro ABI. System will rely on DB or derivation.
	imrSourcedFromChain := false
	// imrMethodName := "getInitialMarginFraction" // Standardized name, could be different in actual ABI
	// imrMethod, imrMethodExists := marginContractABI.Methods[imrMethodName]

	// if imrMethodExists {
	// 	imrArgsToPack := []interface{}{marketID} // Assuming it takes marketID
	// 	imrPackedInput, packErr := imrMethod.Inputs.Pack(imrArgsToPack...)
	// 	if packErr == nil {
	// 		imrCallData := append(imrMethod.ID, imrPackedInput...)
	// 		imrResultBytes, imrCallErr := ethCl.CallContract(context.Background(), hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: imrCallData}, nil)
	// 		if imrCallErr == nil && len(imrResultBytes) > 0 {
	// 			var imrBigInt *big.Int
	// 			// Assuming the method returns a single uint256 value. Adjust if it returns a struct/tuple.
	// 			unpackResults, unpackErr := imrMethod.Outputs.Unpack(imrResultBytes)
	// 			if unpackErr == nil && len(unpackResults) > 0 {
	// 				imrBigInt, ok = unpackResults[0].(*big.Int)
	// 				if ok && imrBigInt != nil {
	// 					imrCandidate := decimal.NewFromBigInt(imrBigInt, -18) // Assuming 1e18 scaled
	// 					if imrCandidate.IsPositive() && imrCandidate.LessThan(decimal.NewFromInt(1)) {
	// 						params.InitialMarginFraction = imrCandidate
	// 						imrSourcedFromChain = true
	// 						utils.Infof("GetMarketMarginParameters for marketID %d: InitialMarginFraction sourced from CHAIN: %s", marketID, params.InitialMarginFraction.String())
	// 					} else {
	// 						utils.Warningf("GetMarketMarginParameters for marketID %d: IMR from chain (%s) is invalid. Will try DB/Derivation.", marketID, imrCandidate.String())
	// 					}
	// 				} else {
	// 					utils.Warningf("GetMarketMarginParameters for marketID %d: Failed to assert IMR from chain to *big.Int. Type was %T. Raw: %s", marketID, unpackResults[0], hexutil.Encode(imrResultBytes))
	// 				}
	// 			} else if unpackErr != nil {
	// 				utils.Warningf("GetMarketMarginParameters for marketID %d: Failed to unpack %s output: %v. Raw: %s", marketID, imrMethodName, unpackErr, hexutil.Encode(imrResultBytes))
	// 			}
	// 		} else if imrCallErr != nil {
	// 			utils.Warningf("GetMarketMarginParameters for marketID %d: Eth_call to %s failed: %v. Will try DB/Derivation.", marketID, imrMethodName, imrCallErr)
	// 		} else if len(imrResultBytes) == 0 && imrMethod.Outputs.Length() > 0 {
	// 			utils.Warningf("GetMarketMarginParameters for marketID %d: Eth_call to %s returned no data. Will try DB/Derivation.", marketID, imrMethodName)
	// 		}
	// 	} else {
	// 		utils.Warningf("GetMarketMarginParameters for marketID %d: Failed to pack args for %s: %v. Will try DB/Derivation.", marketID, imrMethodName, packErr)
	// 	}
	// } else {
	// 	utils.Infof("GetMarketMarginParameters for marketID %d: Method %s not found in ABI. Will try DB/Derivation for IMR.", marketID, imrMethodName)
	// }

	// 3. Attempt to fetch MaintenanceMarginFraction directly from chain.
	// Section for fetching MaintenanceMarginFraction directly from chain commented out as 'getMaintenanceMarginFraction(marketID)'
	// is not found in the provided MarginContractABI or Hydro ABI. System will rely on DB or derivation.
	mmrSourcedFromChain := false
	// mmrMethodName := "getMaintenanceMarginFraction" // Standardized name
	// mmrMethod, mmrMethodExists := marginContractABI.Methods[mmrMethodName]

	// if mmrMethodExists {
	// 	mmrArgsToPack := []interface{}{marketID} // Assuming it takes marketID
	// 	mmrPackedInput, packErr := mmrMethod.Inputs.Pack(mmrArgsToPack...)
	// 	if packErr == nil {
	// 		mmrCallData := append(mmrMethod.ID, mmrPackedInput...)
	// 		mmrResultBytes, mmrCallErr := ethCl.CallContract(context.Background(), hydroSDKCommon.GethCallMsg{To: &MarginContractAddress, Data: mmrCallData}, nil)
	// 		if mmrCallErr == nil && len(mmrResultBytes) > 0 {
	// 			var mmrBigInt *big.Int
	// 			unpackResults, unpackErr := mmrMethod.Outputs.Unpack(mmrResultBytes)
	// 			if unpackErr == nil && len(unpackResults) > 0 {
	// 				mmrBigInt, ok = unpackResults[0].(*big.Int)
	// 				if ok && mmrBigInt != nil {
	// 					mmrCandidate := decimal.NewFromBigInt(mmrBigInt, -18) // Assuming 1e18 scaled
	// 					if mmrCandidate.IsPositive() && mmrCandidate.LessThan(decimal.NewFromInt(1)) {
	// 						params.MaintenanceMarginFraction = mmrCandidate
	// 						mmrSourcedFromChain = true
	// 						utils.Infof("GetMarketMarginParameters for marketID %d: MaintenanceMarginFraction sourced from CHAIN: %s", marketID, params.MaintenanceMarginFraction.String())
	// 					} else {
	// 						utils.Warningf("GetMarketMarginParameters for marketID %d: MMR from chain (%s) is invalid. Will try DB/Derivation.", marketID, mmrCandidate.String())
	// 					}
	// 				} else {
	// 					utils.Warningf("GetMarketMarginParameters for marketID %d: Failed to assert MMR from chain to *big.Int. Type was %T. Raw: %s", marketID, unpackResults[0], hexutil.Encode(mmrResultBytes))
	// 				}
	// 			} else if unpackErr != nil {
	// 				utils.Warningf("GetMarketMarginParameters for marketID %d: Failed to unpack %s output: %v. Raw: %s", marketID, mmrMethodName, unpackErr, hexutil.Encode(mmrResultBytes))
	// 			}
	// 		} else if mmrCallErr != nil {
	// 			utils.Warningf("GetMarketMarginParameters for marketID %d: Eth_call to %s failed: %v. Will try DB/Derivation.", marketID, mmrMethodName, mmrCallErr)
	// 		} else if len(mmrResultBytes) == 0 && mmrMethod.Outputs.Length() > 0 {
	// 			utils.Warningf("GetMarketMarginParameters for marketID %d: Eth_call to %s returned no data. Will try DB/Derivation.", marketID, mmrMethodName)
	// 		}
	// 	} else {
	// 		utils.Warningf("GetMarketMarginParameters for marketID %d: Failed to pack args for %s: %v. Will try DB/Derivation.", marketID, mmrMethodName, packErr)
	// 	}
	// } else {
	// 	utils.Infof("GetMarketMarginParameters for marketID %d: Method %s not found in ABI. Will try DB/Derivation for MMR.", marketID, mmrMethodName)
	// }

	// 4. Fallback to Database if IMR or MMR not successfully sourced from chain.
	var marketDBRecord *models.Market
	var dbErr error
	imrSourcedFromDB := false
	mmrSourcedFromDB := false

	if !imrSourcedFromChain || !mmrSourcedFromChain {
		marketDBRecord, dbErr = models.MarketDaoSql.FindMarketByMarketID(marketID)
		if dbErr != nil {
			utils.Warningf("GetMarketMarginParameters for marketID %d: Error fetching market from DB: %v. Chain values or derivation will be used.", marketID, dbErr)
		} else if marketDBRecord == nil {
			utils.Warningf("GetMarketMarginParameters for marketID %d: Market not found in DB. Chain values or derivation will be used.", marketID)
		}
	}

	if !imrSourcedFromChain && marketDBRecord != nil {
		imrFromDB := marketDBRecord.InitialMarginFraction
		if imrFromDB.IsPositive() && imrFromDB.LessThan(decimal.NewFromInt(1)) {
			params.InitialMarginFraction = imrFromDB
			imrSourcedFromDB = true
			utils.Infof("GetMarketMarginParameters for marketID %d: InitialMarginFraction sourced from DATABASE: %s", marketID, params.InitialMarginFraction.String())
		} else {
			utils.Warningf("GetMarketMarginParameters for marketID %d: IMR from DB (%s) is invalid.", marketID, imrFromDB.String())
		}
	}

	if !mmrSourcedFromChain && marketDBRecord != nil {
		mmrFromDB := marketDBRecord.MaintenanceMarginFraction
		if mmrFromDB.IsPositive() && mmrFromDB.LessThan(decimal.NewFromInt(1)) {
			// If IMR is already sourced (chain or DB), validate MMR < IMR
			currentIMR := params.InitialMarginFraction // Could be from chain or DB (if imrSourcedFromChain or imrSourcedFromDB is true)
			if !currentIMR.IsZero() && mmrFromDB.GreaterThanOrEqual(currentIMR) { // Check if currentIMR is valid before comparing
				utils.Warningf("GetMarketMarginParameters for marketID %d: MMR from DB (%s) is not less than current IMR (%s). Invalidating DB MMR.", marketID, mmrFromDB.String(), currentIMR.String())
			} else {
				params.MaintenanceMarginFraction = mmrFromDB
				mmrSourcedFromDB = true
				utils.Infof("GetMarketMarginParameters for marketID %d: MaintenanceMarginFraction sourced from DATABASE: %s", marketID, params.MaintenanceMarginFraction.String())
			}
		} else {
			utils.Warningf("GetMarketMarginParameters for marketID %d: MMR from DB (%s) is invalid.", marketID, mmrFromDB.String())
		}
	}

	// Determine final source flags for logging
	finalIMRSourced := imrSourcedFromChain || imrSourcedFromDB
	finalMMRSourced := mmrSourcedFromChain || mmrSourcedFromDB

	// 5. Consistency Check & Final Derivation Logic
	// If both are sourced, check IMR > MMR. If not, clear both to trigger full derivation for consistency.
	if finalIMRSourced && finalMMRSourced && params.InitialMarginFraction.LessThanOrEqual(params.MaintenanceMarginFraction) {
		utils.Warningf("GetMarketMarginParameters for marketID %d: Sourced IMR (%s) is NOT GREATER THAN sourced MMR (%s). This is inconsistent. Both will be DERIVED.",
			marketID, params.InitialMarginFraction.String(), params.MaintenanceMarginFraction.String())
		params.InitialMarginFraction = decimal.Zero
		params.MaintenanceMarginFraction = decimal.Zero
		finalIMRSourced = false // Force derivation
		finalMMRSourced = false // Force derivation
	}

	if !finalMMRSourced {
		utils.Warningf("GetMarketMarginParameters for marketID %d: MaintenanceMarginFraction will be DERIVED. TODO: Verify derivation logic.", marketID)
		if params.LiquidateRate.GreaterThan(decimal.NewFromInt(1)) {
			params.MaintenanceMarginFraction = params.LiquidateRate.Sub(decimal.NewFromInt(1))
		} else {
			params.MaintenanceMarginFraction = decimal.NewFromFloat(0.05) // Default if LiquidateRate is not sensible for derivation
			utils.Warningf("GetMarketMarginParameters for marketID %d: LiquidateRate (%s) is <= 1.0. MMR derivation might be incorrect. Defaulting MMR to %s.",
				marketID, params.LiquidateRate.String(), params.MaintenanceMarginFraction.String())
		}
	}

	if !finalIMRSourced {
		utils.Warningf("GetMarketMarginParameters for marketID %d: InitialMarginFraction will be DERIVED. TODO: Verify derivation logic.", marketID)
		// Ensure MMR is set (either sourced or derived by now)
		if params.MaintenanceMarginFraction.IsZero() { // Should not happen if MMR derivation ran
			utils.Error("GetMarketMarginParameters Critical: MMR is zero before IMR derivation. This indicates a logic flaw.")
			// Default MMR again to prevent IMR from being too low or zero.
			if params.LiquidateRate.GreaterThan(decimal.NewFromInt(1)) {
				params.MaintenanceMarginFraction = params.LiquidateRate.Sub(decimal.NewFromInt(1))
			} else {
				params.MaintenanceMarginFraction = decimal.NewFromFloat(0.05)
			}
		}
		params.InitialMarginFraction = params.MaintenanceMarginFraction.Add(decimal.NewFromFloat(0.10)) // Example: IMR = MMR + 10%
	}

	// Final consistency check after any derivation
	if params.InitialMarginFraction.LessThanOrEqual(params.MaintenanceMarginFraction) {
		utils.Criticalf("GetMarketMarginParameters for marketID %d: CRITICAL - Derived IMR (%s) is not greater than MMR (%s). Defaulting to ensure safety.",
			marketID, params.InitialMarginFraction.String(), params.MaintenanceMarginFraction.String())
		// Apply emergency defaults if logic failed to produce valid IMR > MMR
		if params.LiquidateRate.GreaterThan(decimal.NewFromInt(1)) {
			params.MaintenanceMarginFraction = params.LiquidateRate.Sub(decimal.NewFromInt(1))
		} else {
			params.MaintenanceMarginFraction = decimal.NewFromFloat(0.05)
		}
		params.InitialMarginFraction = params.MaintenanceMarginFraction.Add(decimal.NewFromFloat(0.05)) // Smaller gap for safety default
	}

	imrSource := getParamSource(imrSourcedFromChain, imrSourcedFromDB, !finalIMRSourced || params.InitialMarginFraction.IsZero())
	mmrSource := getParamSource(mmrSourcedFromChain, mmrSourcedFromDB, !finalMMRSourced || params.MaintenanceMarginFraction.IsZero())

	utils.Infof("GetMarketMarginParameters Final for marketID %d: LiquidateRate (Chain): %s, IMR (%s): %s, MMR (%s): %s, BorrowEnable: %t.",
		marketID, params.LiquidateRate.String(),
		imrSource, params.InitialMarginFraction.String(),
		mmrSource, params.MaintenanceMarginFraction.String(),
		marketDataTuple.BorrowEnable)

	return params, nil
}

// Helper function to describe parameter source for logging
func getParamSource(isChain bool, isDB bool, isDerived bool) string {
	if isChain {
		return "Chain"
	}
	if isDB {
		return "Database"
	}
	// isDerived flag explicitly passed, or if neither chain nor DB, it's derived.
	return "Derived"
}


// --- ERC20 Balance Wrapper ---

var erc20BalanceOfABI abi.ABI
var erc20DecimalsABI abi.ABI // For decimals()

func init() {
	// Minimal ERC20 ABI for balanceOf
	erc20BalanceOfAbiJson := `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]`
	var err error
	erc20BalanceOfABI, err = abi.JSON(strings.NewReader(erc20BalanceOfAbiJson))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse minimal ERC20 ABI for balanceOf: %v", err))
	}

	// Minimal ERC20 ABI for decimals
	erc20DecimalsAbiJson := `[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`
	erc20DecimalsABI, err = abi.JSON(strings.NewReader(erc20DecimalsAbiJson))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse minimal ERC20 ABI for decimals: %v", err))
	}
}

// GetTokenDecimals retrieves the number of decimals for an ERC20 token by making an actual contract call.
func GetTokenDecimals(hydro sdk.Hydro, tokenAddress goEthereumCommon.Address) (int32, error) {
	if len(erc20DecimalsABI.Methods) == 0 {
		return 0, fmt.Errorf("GetTokenDecimals: ERC20 decimals ABI not initialized")
	}

	methodName := "decimals"
	method, ok := erc20DecimalsABI.Methods[methodName]
	if !ok {
		return 0, fmt.Errorf("GetTokenDecimals: method %s not found in ERC20 decimals ABI", methodName)
	}

	// No arguments for decimals() method, so data is just the method ID.
	data := method.ID

	hydroEth, okEth := hydro.(*ethereum.EthereumHydro)
	if !okEth || hydroEth.EthClient() == nil {
		return 0, fmt.Errorf("GetTokenDecimals: hydro SDK object does not provide a usable Ethereum client")
	}
	ethCl := hydroEth.EthClient()

	callMsg := hydroSDKCommon.GethCallMsg{To: &tokenAddress, Data: data}
	responseData, callErr := ethCl.CallContract(context.Background(), callMsg, nil)
	if callErr != nil {
		return 0, fmt.Errorf("GetTokenDecimals: ERC20 %s call failed for token %s: %w", methodName, tokenAddress.Hex(), callErr)
	}

	if len(responseData) == 0 && method.Outputs.Length() > 0 {
        return 0, fmt.Errorf("GetTokenDecimals: ERC20 %s call returned no data for token %s, but expected %d outputs", methodName, tokenAddress.Hex(), method.Outputs.Length())
    }

	results, err := method.Outputs.Unpack(responseData)
	if err != nil {
		return 0, fmt.Errorf("GetTokenDecimals: failed to unpack %s result for token %s: %w. Raw data: %s", methodName, tokenAddress.Hex(), err, hexutil.Encode(responseData))
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("GetTokenDecimals: no output returned from %s for token %s", methodName, tokenAddress.Hex())
	}

	decimalsUint8, ok := results[0].(uint8)
	if !ok {
		return 0, fmt.Errorf("GetTokenDecimals: %s output for token %s is not uint8, type is %T. Value: %v", methodName, tokenAddress.Hex(), results[0], results[0])
	}

	return int32(decimalsUint8), nil
}

// EncodeLiquidateAccountParamsForBatch ABI-encodes parameters for a liquidateAccount action within a batch.
// Solidity signature (example, assuming it's part of batch actions): liquidateAccount(address user, uint16 marketID)
// func EncodeLiquidateAccountParamsForBatch(userToLiquidate goEthereumCommon.Address, marketID uint16) ([]byte, error) {
// 	// addressType, _ := abi.NewType("address", "", nil)
// 	// uint16Type, _ := abi.NewType("uint16", "", nil)
// 	// arguments := abi.Arguments{{Type: addressType, Name: "userToLiquidate"}, {Type: uint16Type, Name: "marketID"}}
// 	// packedBytes, err := arguments.Pack(userToLiquidate, marketID)
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("failed to pack liquidateAccount params for batch: %v", err)
// 	// }
// 	// utils.LogWarnf("EncodeLiquidateAccountParamsForBatch is a conceptual placeholder. Actual BatchActions encoding might differ if liquidateAccount is a standard ActionType.")
// 	// return packedBytes, nil
// 	utils.Warningf("EncodeLiquidateAccountParamsForBatch is a placeholder and not fully implemented. Its usage depends on whether liquidateAccount is a standard batch action type or a direct contract method.")
// 	return nil, fmt.Errorf("EncodeLiquidateAccountParamsForBatch not fully implemented")
// }

// PrepareLiquidateAccountDirectTransaction prepares the data for a direct call to liquidateAccount on the Margin Contract.
// It returns placeholder data for nonce, gas, and chainID, which must be implemented.
func PrepareLiquidateAccountDirectTransaction(
	hydro sdk.Hydro,
	liquidatorAddress goEthereumCommon.Address, // The EOA that will send the liquidation transaction
	userToLiquidate goEthereumCommon.Address,
	marketID uint16,
	// value *big.Int, // liquidateAccount is non-payable as per provided ABI
) (*UnsignedTxDataForClient, error) {
	// utils.Warningf("PrepareLiquidateAccountDirectTransaction is a placeholder and needs full implementation for nonce, gas, and chainID fetching for the liquidatorAddress: %s", liquidatorAddress.Hex())

	if marginContractABI.Methods == nil {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: Margin Contract ABI not initialized")
	}
	methodName := "liquidateAccount"
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: method '%s' not found in Margin Contract ABI", methodName)
	}

	packedArgs, err := method.Inputs.Pack(userToLiquidate, marketID)
	if err != nil {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: failed to pack args for '%s': %w", methodName, err)
	}
	fullTxData := append(method.ID, packedArgs...)

	hydroEth, okEth := hydro.(*ethereum.EthereumHydro)
	if !okEth {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: hydro SDK object cannot be asserted to *ethereum.EthereumHydro")
	}
	ethCl := hydroEth.EthClient()
	if ethCl == nil {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: EthClient from hydro SDK is nil")
	}

	currentNonce, err := ethCl.PendingNonceAt(context.Background(), liquidatorAddress)
	if err != nil {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: failed to get nonce for liquidator %s: %w", liquidatorAddress.Hex(), err)
	}

	currentGasPrice, err := ethCl.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: failed to suggest gas price: %w", err)
	}

	txValue := big.NewInt(0) // liquidateAccount is non-payable
	callMsg := hydroSDKCommon.GethCallMsg{
		From:  liquidatorAddress,
		To:    &MarginContractAddress,
		Data:  fullTxData,
		Value: txValue,
	}
	estimatedGasLimit, err := ethCl.EstimateGas(context.Background(), callMsg)
	if err != nil {
		utils.Errorf("PrepareLiquidateAccountDirectTransaction: Gas estimation failed for liquidator %s, user %s, market %d. Data: %s, Error: %v",
			liquidatorAddress.Hex(), userToLiquidate.Hex(), marketID, hexutil.Encode(fullTxData), err)
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: failed to estimate gas: %w", err)
	}
	estimatedGasLimit = estimatedGasLimit + (estimatedGasLimit / 5) // Add 20% buffer

	currentChainID, err := ethCl.NetworkID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("PrepareLiquidateAccountDirectTransaction: failed to get network ID: %w", err)
	}

	utils.Infof("PrepareLiquidateAccountDirectTransaction: TX prepared for liquidator %s to liquidate user %s in market %d. Data: %s.",
		liquidatorAddress.Hex(), userToLiquidate.Hex(), marketID, hexutil.Encode(fullTxData))

	return &UnsignedTxDataForClient{
		From:     liquidatorAddress.Hex(),
		To:       MarginContractAddress.Hex(),
		Nonce:    currentNonce,
		GasPrice: currentGasPrice.String(),
		GasLimit: estimatedGasLimit,
		Value:    txValue.String(),
		Data:     hexutil.Encode(fullTxData),
		ChainID:  currentChainID.String(),
	}, nil
}

// GetOraclePriceInQuote fetches the price of assetToPrice in terms of quoteAsset by using their respective USD prices.
// E.g., how many quoteAssets is one unit of assetToPrice worth? (assetToPrice_USD / quoteAsset_USD)
func GetOraclePriceInQuote(hydro sdk.Hydro, assetToPriceAddress goEthereumCommon.Address, quoteAssetAddress goEthereumCommon.Address) (decimal.Decimal, error) {
	if assetToPriceAddress == quoteAssetAddress {
		return decimal.NewFromInt(1), nil // Price of an asset in terms of itself is 1
	}

	// Fetch USD price of assetToPrice
	assetToPriceInfo, err := GetAsset(hydro, assetToPriceAddress)
	if err != nil {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: failed to get asset info for assetToPrice %s: %w", assetToPriceAddress.Hex(), err)
	}
	if assetToPriceInfo.PriceOracle == (goEthereumCommon.Address{}) {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: no price oracle configured for assetToPrice %s", assetToPriceAddress.Hex())
	}
	priceAssetToPriceUSD_bigInt, err := GetOraclePrice(hydro, assetToPriceInfo.PriceOracle, assetToPriceAddress)
	if err != nil {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: failed to get USD price for assetToPrice %s from oracle %s: %w", assetToPriceAddress.Hex(), assetToPriceInfo.PriceOracle.Hex(), err)
	}
	// Assuming oracle prices are returned with 18 decimals for USD value
	priceAssetToPriceUSD_dec := utils.BigIntToDecimal(priceAssetToPriceUSD_bigInt, 18)
	if priceAssetToPriceUSD_dec.IsZero() {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: USD price for assetToPrice %s is zero", assetToPriceAddress.Hex())
	}

	// Fetch USD price of quoteAsset
	quoteAssetInfo, err := GetAsset(hydro, quoteAssetAddress)
	if err != nil {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: failed to get asset info for quoteAsset %s: %w", quoteAssetAddress.Hex(), err)
	}
	if quoteAssetInfo.PriceOracle == (goEthereumCommon.Address{}) {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: no price oracle configured for quoteAsset %s", quoteAssetAddress.Hex())
	}
	priceQuoteAssetUSD_bigInt, err := GetOraclePrice(hydro, quoteAssetInfo.PriceOracle, quoteAssetAddress)
	if err != nil {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: failed to get USD price for quoteAsset %s from oracle %s: %w", quoteAssetAddress.Hex(), quoteAssetInfo.PriceOracle.Hex(), err)
	}
	priceQuoteAssetUSD_dec := utils.BigIntToDecimal(priceQuoteAssetUSD_bigInt, 18)
	if priceQuoteAssetUSD_dec.IsZero() {
		return decimal.Zero, fmt.Errorf("GetOraclePriceInQuote: USD price for quoteAsset %s is zero, cannot calculate relative price", quoteAssetAddress.Hex())
	}

	// Calculate relative price: assetToPrice_USD / quoteAsset_USD
	relativePrice := priceAssetToPriceUSD_dec.Div(priceQuoteAssetUSD_dec)
	utils.Infof("GetOraclePriceInQuote: Asset %s (USD %s) in terms of Asset %s (USD %s) = %s",
		assetToPriceAddress.Hex(), priceAssetToPriceUSD_dec.String(),
		quoteAssetAddress.Hex(), priceQuoteAssetUSD_dec.String(),
		relativePrice.String())
	return relativePrice, nil
}

// BalanceOf retrieves the ERC20 token balance for a user by making an actual contract call.
// It uses a mocked GetTokenDecimals for now.
func BalanceOf(hydro sdk.Hydro, tokenAddress goEthereumCommon.Address, userAddress goEthereumCommon.Address) (decimal.Decimal, error) {
	if len(erc20BalanceOfABI.Methods) == 0 {
		return decimal.Zero, fmt.Errorf("BalanceOf: ERC20 balanceOf ABI not initialized")
	}

	methodName := "balanceOf"
	method, ok := erc20BalanceOfABI.Methods[methodName]
	if !ok {
		return decimal.Zero, fmt.Errorf("BalanceOf: method %s not found in ERC20 balanceOf ABI", methodName)
	}

	argsToPack := []interface{}{userAddress}
	packedArgs, err := method.Inputs.Pack(argsToPack...)
	if err != nil {
		return decimal.Zero, fmt.Errorf("BalanceOf: failed to pack args for %s for token %s, user %s: %w", methodName, tokenAddress.Hex(), userAddress.Hex(), err)
	}

	data := append(method.ID, packedArgs...)

	hydroEth, okEth := hydro.(*ethereum.EthereumHydro)
	if !okEth || hydroEth.EthClient() == nil {
		return decimal.Zero, fmt.Errorf("BalanceOf: hydro SDK object does not provide a usable Ethereum client")
	}
	ethCl := hydroEth.EthClient()

	callMsg := hydroSDKCommon.GethCallMsg{To: &tokenAddress, Data: data}
	responseData, callErr := ethCl.CallContract(context.Background(), callMsg, nil)
	if callErr != nil {
		return decimal.Zero, fmt.Errorf("BalanceOf: ERC20 %s call failed for token %s, user %s: %w", methodName, tokenAddress.Hex(), userAddress.Hex(), callErr)
	}

	if len(responseData) == 0 && method.Outputs.Length() > 0 {
		return decimal.Zero, fmt.Errorf("BalanceOf: ERC20 %s call returned no data for token %s, user %s, but expected %d outputs", methodName, tokenAddress.Hex(), userAddress.Hex(), method.Outputs.Length())
	}

	results, err := method.Outputs.Unpack(responseData)
	if err != nil {
		return decimal.Zero, fmt.Errorf("BalanceOf: failed to unpack %s result for token %s, user %s: %w. Raw data: %s", methodName, tokenAddress.Hex(), userAddress.Hex(), err, hexutil.Encode(responseData))
	}

	if len(results) == 0 {
		return decimal.Zero, fmt.Errorf("BalanceOf: no output returned from %s for token %s, user %s", methodName, tokenAddress.Hex(), userAddress.Hex())
	}

	balanceBigInt, ok := results[0].(*big.Int)
	if !ok {
		return decimal.Zero, fmt.Errorf("BalanceOf: %s result for token %s, user %s is not *big.Int, type is %T. Value: %v", methodName, tokenAddress.Hex(), userAddress.Hex(), results[0], results[0])
	}

	// Get token decimals (currently mocked)
	tokenDecimals, err := GetTokenDecimals(hydro, tokenAddress)
	if err != nil {
		utils.Warningf("BalanceOf: Failed to get decimals for token %s: %v. Defaulting to 18.", tokenAddress.Hex(), err)
		tokenDecimals = 18 // Default to 18 if fetching decimals fails
	}

	return decimal.NewFromBigInt(balanceBigInt, -tokenDecimals), nil
}

// UnsignedTxDataForClient holds the data for a transaction to be signed by the client.
// Ensure this matches the structure expected by the frontend for signing.
type UnsignedTxDataForClient struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Nonce    uint64 `json:"nonce"`
	GasPrice string `json:"gasPrice"` // string for big.Int
	GasLimit uint64 `json:"gasLimit"`
	Value    string `json:"value"`    // string for big.Int
	Data     string `json:"data"`     // hex string
	ChainID  string `json:"chainId"`  // string for big.Int
}

// PrepareBatchActionsTransaction prepares the data needed for the client to sign and send a batch transaction.
// It fetches the nonce, suggests gas price, estimates gas limit, and gets the chain ID.
func PrepareBatchActionsTransaction(hydro sdk.Hydro, actions []SDKBatchAction, userAddress goEthereumCommon.Address, value *big.Int) (*UnsignedTxDataForClient, error) {
	if MarginContractAddress == (goEthereumCommon.Address{}) {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: MarginContractAddress not initialized")
	}
	if len(marginContractABI.Methods) == 0 {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: Margin Contract ABI not initialized")
	}

	methodName := "batch"
	method, ok := marginContractABI.Methods[methodName]
	if !ok {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: method %s not found in Margin Contract ABI", methodName)
	}

	// Pack the actions for the 'batch' call data
	fullTxData, err := method.Inputs.Pack(actions)
	if err != nil {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: failed to pack actions for %s: %w. Actions: %+v", methodName, err, actions)
	}
	// The actual data sent in the transaction is methodID + packed arguments
	fullTxData = append(method.ID, fullTxData...)


	hydroEth, okEth := hydro.(*ethereum.EthereumHydro)
	if !okEth || hydroEth.EthClient() == nil {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: hydro SDK object does not provide a usable Ethereum client")
	}
	ethCl := hydroEth.EthClient()

	// 1. Fetch Nonce
	nonce, err := ethCl.PendingNonceAt(context.Background(), userAddress)
	if err != nil {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: failed to get nonce for %s: %w", userAddress.Hex(), err)
	}

	// 2. Suggest Gas Price
	gasPrice, err := ethCl.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: failed to suggest gas price: %w", err)
	}

	// 3. Estimate Gas Limit
	callMsg := hydroSDKCommon.GethCallMsg{
		From:  userAddress,            // User's address
		To:    &MarginContractAddress, // Target contract
		Data:  fullTxData,             // MethodID + packed arguments
		Value: value,                  // ETH value to send with the transaction
	}

	gasLimit, err := ethCl.EstimateGas(context.Background(), callMsg)
	if err != nil {
		// Log details for debugging gas estimation failures
		utils.Errorf("PrepareBatchActionsTransaction: Gas estimation failed. User: %s, To: %s, Value: %s, Data: %s, Error: %v",
			userAddress.Hex(), MarginContractAddress.Hex(), value.String(), hexutil.Encode(fullTxData), err)
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: failed to estimate gas: %w", err)
	}
	// Add a buffer to the gas limit (e.g., 20%)
	gasLimit = gasLimit + (gasLimit / 5)

	// 4. Fetch ChainID
	chainID, err := ethCl.NetworkID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("PrepareBatchActionsTransaction: failed to get network ID: %w", err)
	}

	// 5. Populate UnsignedTxDataForClient
	return &UnsignedTxDataForClient{
		From:     userAddress.Hex(),
		To:       MarginContractAddress.Hex(),
		Nonce:    nonce,
		GasPrice: gasPrice.String(),
		GasLimit: gasLimit,
		Value:    value.String(),
		Data:     hexutil.Encode(fullTxData),
		ChainID:  chainID.String(),
	}, nil
}
