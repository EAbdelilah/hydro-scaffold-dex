package api

import "github.com/shopspring/decimal"

// MarginAssetDetails defines the structure for asset details within a margin account.
type MarginAssetDetails struct {
	AssetAddress       string          `json:"assetAddress"`
	Symbol             string          `json:"symbol"`
	TotalBalance       decimal.Decimal `json:"totalBalance"` // Total collateral deposited for this asset
	TransferableAmount decimal.Decimal `json:"transferableAmount"` // Amount that can be withdrawn
}

// MarginAccountDetailsResp defines the response structure for margin account details.
type MarginAccountDetailsResp struct {
	MarketID            string               `json:"marketID"`
	UserAddress         string               `json:"userAddress"`
	Liquidatable        bool                 `json:"liquidatable"`
	Status              string               `json:"status"` // e.g., "Normal", "Liquidated", "MarginCall"
	DebtsTotalUSDValue  decimal.Decimal      `json:"debtsTotalUSDValue"`
	AssetsTotalUSDValue decimal.Decimal      `json:"assetsTotalUSDValue"` // This is Collateral Value
	CollateralRatio     decimal.Decimal      `json:"collateralRatio,omitempty"` // Only if DebtsTotalUSDValue > 0
	BaseAssetDetails    MarginAssetDetails   `json:"baseAssetDetails"`
	QuoteAssetDetails   MarginAssetDetails   `json:"quoteAssetDetails"`
}

// MarginAccountDetailsReq defines the request structure for fetching margin account details.
// UserAddress will eventually be replaced by authenticated user context.
type MarginAccountDetailsReq struct {
	BaseReq     // Embed BaseReq for potential auth or common fields if needed in future
	MarketID    string `param:"marketID" validate:"required"`
	UserAddress string `query:"user" validate:"required,eth_addr"` // TODO: Replace with authenticated user from context
}

// CollateralManagementReq defines the request structure for depositing or withdrawing collateral.
type CollateralManagementReq struct {
	BaseReq      // Embed BaseReq for auth
	MarketID     string          `json:"marketID" validate:"required"`
	AssetAddress string          `json:"assetAddress" validate:"required,eth_addr"`
	Amount       decimal.Decimal `json:"amount" validate:"required"` // Validation for >0 should be done in handler
}

// Ensure Param interface is satisfied for requests that need it (like MarginAccountDetailsReq)
func (r *MarginAccountDetailsReq) GetAddress() string {
	// TODO: This will be from auth context eventually. For now, using UserAddress field for explicit query.
	// For requests that don't take user address as param but rely on auth, BaseReq.Address will be set by middleware.
	return r.UserAddress
}

// SetAddress is part of the Param interface.
// For MarginAccountDetailsReq, UserAddress from query param is the primary source for now.
// BaseReq.Address might be set by auth middleware later.
func (r *MarginAccountDetailsReq) SetAddress(address string) {
	r.BaseReq.Address = address
	// If UserAddress is empty and address is provided (e.g. from auth), set it.
	// However, current validation requires UserAddress query param.
	// This logic might need adjustment based on final auth flow.
	if r.UserAddress == "" {
		r.UserAddress = address
	}
}

// --- Loan Management Types ---

// LoanDetails defines the structure for details of a single loan.
type LoanDetails struct {
	AssetAddress   string          `json:"assetAddress"`
	Symbol         string          `json:"symbol"`
	AmountBorrowed decimal.Decimal `json:"amountBorrowed"`
	// CurrentInterestRate decimal.Decimal `json:"currentInterestRate,omitempty"` // Optional: To be added if readily available from SDK/contract state
}

// LoanListResp defines the response for listing a user's loans in a market.
type LoanListResp struct {
	MarketID    string        `json:"marketID"`
	UserAddress string        `json:"userAddress"`
	Loans       []LoanDetails `json:"loans"`
}

// LoanListReq defines the request query parameters for fetching a user's loans.
// UserAddress will eventually be replaced by authenticated user context.
type LoanListReq struct {
	BaseReq
	MarketID    string `query:"marketID" validate:"required"`
	UserAddress string `query:"user" validate:"required,eth_addr"` // TODO: Replace with authenticated user from context
}

// GetAddress implements the Param interface for LoanListReq.
func (r *LoanListReq) GetAddress() string {
	// TODO: This will be from auth context eventually.
	return r.UserAddress
}

// SetAddress implements the Param interface for LoanListReq.
func (r *LoanListReq) SetAddress(address string) {
	r.BaseReq.Address = address
	if r.UserAddress == "" {
		r.UserAddress = address
	}
}
