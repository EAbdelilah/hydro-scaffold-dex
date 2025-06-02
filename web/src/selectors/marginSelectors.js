import { createSelector } from 'reselect';
import { fromJS, Map, List } from 'immutable'; // Assuming immutable state
import { BigNumber } from 'bignumber.js'; // For calculations

// Helper to get the margin state slice
export const getMarginState = state => state.margin || Map(); // Default to an empty Map if not present

// --- UI State Selectors ---
export const getSelectedAccountType = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'selectedAccountType'], 'spot') // Default to 'spot'
);

// Deposit Collateral
export const isDepositingCollateral = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'depositCollateralLoading'], false)
);
export const getDepositCollateralError = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'depositCollateralError'], null)
);

// Withdraw Collateral
export const isWithdrawingCollateral = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'withdrawCollateralLoading'], false)
);
export const getWithdrawCollateralError = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'withdrawCollateralError'], null)
);

// Borrow Loan
export const isBorrowingLoan = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'borrowLoanLoading'], false)
);
export const getBorrowLoanError = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'borrowLoanError'], null)
);

// Repay Loan
export const isRepayingLoan = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'repayLoanLoading'], false)
);
export const getRepayLoanError = createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', 'repayLoanError'], null)
);


// --- Per-Market Data Selectors ---

// Margin Account Details
export const getAccountDetailsByMarketMap = createSelector(
  getMarginState,
  marginState => marginState.get('accountDetailsByMarket', Map())
);

// Selector that takes marketID as an argument
export const getMarginAccountDetailsForMarket = createSelector(
  [getAccountDetailsByMarketMap, (state, marketID) => marketID],
  (accountDetailsMap, marketID) => accountDetailsMap.get(marketID, Map()) // Default to empty Map
);

// Further derived selector to get the 'details' sub-object (actual response data)
export const getMarginAccountDetailsData = createSelector(
  getMarginAccountDetailsForMarket, // Uses the marketID-specific selector
  details => details || Map() // Ensure it returns a Map, not potentially undefined from .get if details is not there
);

export const getMarginAccountDetailsLoading = createSelector(
  getMarginState,
  (state, marketID) => marketID,
  (marginState, marketID) => marginState.getIn(['ui', 'getMarginAccountDetailsLoading', marketID], false)
);

export const getMarginAccountDetailsError = createSelector(
  getMarginState,
  (state, marketID) => marketID,
  (marginState, marketID) => marginState.getIn(['ui', 'getMarginAccountDetailsError', marketID], null)
);

// Loans
export const getLoansByMarketMap = createSelector(
  getMarginState,
  marginState => marginState.get('loansByMarket', Map())
);

// Selector that takes marketID as an argument
export const getLoansForMarket = createSelector(
  [getLoansByMarketMap, (state, marketID) => marketID],
  (loansMap, marketID) => {
    const marketLoans = loansMap.get(marketID, Map()); // Get the map for the market
    return marketLoans.get('loans', List()); // Get the 'loans' list, default to empty List
  }
);

export const getLoansLoading = createSelector(
  getMarginState,
  (state, marketID) => marketID,
  (marginState, marketID) => marginState.getIn(['ui', 'getLoansLoading', marketID], false)
);

export const getLoansError = createSelector(
  getMarginState,
  (state, marketID) => marketID,
  (marginState, marketID) => marginState.getIn(['ui', 'getLoansError', marketID], null)
);


// --- Derived Data Selectors (Examples) ---

// getCollateralBalance requires the *specific market's details* and an assetSymbol
export const getCollateralBalance = createSelector(
  getMarginAccountDetailsData, // This selector already takes (state, marketID)
  (state, marketID, assetSymbol) => assetSymbol,
  (details, assetSymbol) => {
    if (!details || details.isEmpty()) return new BigNumber(0);
    const baseDetails = details.get('baseAssetDetails', Map());
    const quoteDetails = details.get('quoteAssetDetails', Map());

    if (baseDetails.get('symbol') === assetSymbol) {
      return new BigNumber(baseDetails.get('totalBalance', '0'));
    }
    if (quoteDetails.get('symbol') === assetSymbol) {
      return new BigNumber(quoteDetails.get('totalBalance', '0'));
    }
    return new BigNumber(0);
  }
);

// getBorrowedAmount requires the *specific market's loans* and an assetSymbol
export const getBorrowedAmount = createSelector(
  getLoansForMarket, // This selector already takes (state, marketID)
  (state, marketID, assetSymbol) => assetSymbol,
  (loans, assetSymbol) => {
    if (!loans || loans.isEmpty()) return new BigNumber(0);
    const loan = loans.find(l => l.get('symbol') === assetSymbol);
    return loan ? new BigNumber(loan.get('amountBorrowed', '0')) : new BigNumber(0);
  }
);

// Selector for the raw map of spendable balances
export const getMarginSpendableBalancesMap = createSelector(
  getMarginState,
  marginState => marginState.get('marginSpendableBalances', Map())
);

// Selector for a specific market's spendable balances (map of assetSymbol -> amount)
export const getSpendableBalancesForMarket = createSelector(
  [getMarginSpendableBalancesMap, (state, marketID) => marketID],
  (balancesMap, marketID) => balancesMap.get(marketID, Map())
);

// getSpendableMarginBalance retrieves the transferable/spendable balance for a specific asset in a market.
// This corresponds to data fetched by fetchSpendableMarginBalance action.
export const getSpendableMarginBalance = createSelector(
  [getSpendableBalancesForMarket, (state, marketID, assetSymbol) => assetSymbol],
  (marketBalances, assetSymbol) => new BigNumber(marketBalances.get(assetSymbol, '0')) // Default to '0' if not found
);

export const getSpendableMarginBalanceLoading = createSelector(
  getMarginState,
  (state, marketID, assetSymbol) => marketID,
  (state, marketID, assetSymbol) => assetSymbol,
  (marginState, marketID, assetSymbol) =>
    marginState.getIn(['ui', 'fetchSpendableMarginBalanceLoading', marketID, assetSymbol], false)
);

export const getSpendableMarginBalanceError = createSelector(
  getMarginState,
  (state, marketID, assetSymbol) => marketID,
  (state, marketID, assetSymbol) => assetSymbol,
  (marginState, marketID, assetSymbol) =>
    marginState.getIn(['ui', 'fetchSpendableMarginBalanceError', marketID, assetSymbol], null)
);


export const getCollateralRatio = createSelector(
  getMarginAccountDetailsData, // This selector takes (state, marketID)
  (details) => {
    if (!details || details.isEmpty()) return null;
    const assetsUSD = new BigNumber(details.get('assetsTotalUSDValue', '0'));
    const debtsUSD = new BigNumber(details.get('debtsTotalUSDValue', '0'));
    if (debtsUSD.isZero()) {
      // If there are assets but no debt, collateral ratio is effectively infinite or undefined.
      // If no assets and no debt, it's also undefined or could be considered 0 or 1.
      // Returning null is a common way to indicate it's not meaningfully calculable.
      return assetsUSD.isZero() ? null : new BigNumber(Infinity) ;
    }
    return assetsUSD.div(debtsUSD);
  }
);

// TODO: Add getLiquidationPrice selector - requires oracle prices and position details.
// This would be more complex, involving:
// - Total debt of specific asset (e.g., debtQuoteToken)
// - Total collateral of other asset (e.g., collateralBaseToken)
// - Market's liquidation rate (e.g., market.liquidateRate)
// - Formula: LiquidationPrice_Base = (debtQuoteToken * liquidateRate) / collateralBaseToken (if base is being liquidated to cover quote debt)
// - Or: LiquidationPrice_Quote = collateralQuoteToken / (debtBaseToken * liquidateRate) (if quote is being liquidated to cover base debt)
// This needs careful thought on which asset is the debt and which is collateral for a given liquidation scenario.
// It often depends on the net debt value in USD vs collateral value in USD.
// For now, components can get assetsTotalUSDValue and debtsTotalUSDValue and market.liquidateRate
// and try to estimate it, or it comes from backend if the backend calculates it.

// Selector for specific loading state (example for one action, others follow same pattern)
export const isActionLoading = (actionPrefix) => createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', `${actionPrefix}Loading`], false)
);

export const getActionError = (actionPrefix) => createSelector(
  getMarginState,
  marginState => marginState.getIn(['ui', `${actionPrefix}Error`], null)
);

// --- Open Positions Selectors ---
export const getOpenPositionsState = createSelector(
  getMarginState,
  marginState => marginState.get('openPositions', Map({ list: List(), isLoading: false, error: null }))
);

export const getOpenPositionsList = createSelector(
  getOpenPositionsState,
  openPositionsState => openPositionsState.get('list', List())
);

export const getOpenPositionsLoading = createSelector(
  getOpenPositionsState,
  openPositionsState => openPositionsState.get('isLoading', false)
);

export const getOpenPositionsError = createSelector(
  getOpenPositionsState,
  openPositionsState => openPositionsState.get('error', null)
);

// --- Active Loans Selectors (can be enhanced or used with getLoansForMarket) ---
// getLoansForMarket (already exists) is good for market-specific views.
// For a global list of all active loans for a user across all markets:
export const getAllActiveLoansList = createSelector(
  getLoansByMarketMap, // Existing selector: returns Map({ marketID1: {loans: List(...) }, marketID2: ... })
  (loansByMarketMap) => {
    let allLoans = List();
    loansByMarketMap.forEach(marketLoansData => {
      // marketLoansData is expected to be a Map with a 'loans' key holding a List
      const loansInMarket = marketLoansData.get('loans', List());
      allLoans = allLoans.concat(loansInMarket);
    });
    return allLoans;
  }
);

// General loading state for any loan fetching (if needed, or use market-specific getLoansLoading)
// This might need a more sophisticated way to determine if *any* loan fetching is in progress
// if fetchLoans is always market-specific. For now, this is a conceptual placeholder.
// A simpler approach is that the component using this would iterate markets and use getLoansLoading for each.
// However, if there was a global "fetch all loans" action, this would be its loading state.
export const getAnyLoanLoadingState = createSelector(
    getMarginState,
    marginState => {
        const loansLoadingMap = marginState.getIn(['ui', 'getLoansLoading'], Map());
        return loansLoadingMap.some(isLoading => isLoading === true);
    }
);
