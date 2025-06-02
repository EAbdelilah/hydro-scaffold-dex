import { fromJS, List } from 'immutable'; // Added List
import {
  SELECT_ACCOUNT_TYPE,
  FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST,
  FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS,
  FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE,
  DEPOSIT_COLLATERAL_REQUEST,
  DEPOSIT_COLLATERAL_SUCCESS,
  DEPOSIT_COLLATERAL_FAILURE,
  WITHDRAW_COLLATERAL_REQUEST,
  WITHDRAW_COLLATERAL_SUCCESS,
  WITHDRAW_COLLATERAL_FAILURE,
  BORROW_LOAN_REQUEST,
  BORROW_LOAN_SUCCESS,
  BORROW_LOAN_FAILURE,
  REPAY_LOAN_REQUEST,
  REPAY_LOAN_SUCCESS,
  REPAY_LOAN_FAILURE,
  FETCH_LOANS_REQUEST,
  FETCH_LOANS_SUCCESS,
  FETCH_LOANS_FAILURE,
  FETCH_SPENDABLE_MARGIN_BALANCE_REQUEST,
  FETCH_SPENDABLE_MARGIN_BALANCE_SUCCESS,
  FETCH_SPENDABLE_MARGIN_BALANCE_FAILURE,
  HANDLE_MARGIN_ACCOUNT_UPDATE, // Imported new action type
  HANDLE_MARGIN_ALERT,          // Imported new action type
  DISMISS_MARGIN_ALERT          // Imported new action type
  // HANDLE_AUCTION_UPDATE // Keep for future
} from '../actions/marginActions';

const initialState = fromJS({
  accountDetailsByMarket: {}, // Keyed by marketID. Stores MarginAccountDetailsResp for each market.
  loansByMarket: {},          // Keyed by marketID. Stores array of loans for each market.
  marginSpendableBalances: {}, // Keyed by marketID, then assetSymbol. Stores string amount.
  // auctions: {}, // Future use for auction data, keyed by auctionID
  ui: {
    selectedAccountType: 'spot', // "spot" or "margin" - global toggle for trading context

    getMarginAccountDetailsLoading: {},
    getMarginAccountDetailsError: {},
    getLoansLoading: {},
    getLoansError: {},

    depositCollateralLoading: false,
    depositCollateralError: null,
    withdrawCollateralLoading: false,
    withdrawCollateralError: null,
    borrowLoanLoading: false,
    borrowLoanError: null,
    repayLoanLoading: false,
    repayLoanError: null,

    activeMarginAlerts: List(), // Initialize as an empty Immutable List
  }
});

export default function marginReducer(state = initialState, action) {
  switch (action.type) {
    case SELECT_ACCOUNT_TYPE:
      return state.setIn(['ui', 'selectedAccountType'], action.payload.accountType);

    case FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST:
      return state
        .setIn(['ui', 'getMarginAccountDetailsLoading', action.payload.marketID], true)
        .setIn(['ui', 'getMarginAccountDetailsError', action.payload.marketID], null);
    case FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS:
      return state
        .setIn(['ui', 'getMarginAccountDetailsLoading', action.payload.marketID], false)
        .setIn(['accountDetailsByMarket', action.payload.marketID], fromJS(action.payload.details || {}));
    case FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE:
      return state
        .setIn(['ui', 'getMarginAccountDetailsLoading', action.payload.marketID], false)
        .setIn(['ui', 'getMarginAccountDetailsError', action.payload.marketID], action.payload.error);

    case FETCH_LOANS_REQUEST:
      return state
        .setIn(['ui', 'getLoansLoading', action.payload.marketID], true)
        .setIn(['ui', 'getLoansError', action.payload.marketID], null);
    case FETCH_LOANS_SUCCESS:
      return state
        .setIn(['ui', 'getLoansLoading', action.payload.marketID], false)
        .setIn(['loansByMarket', action.payload.marketID], fromJS(action.payload.loans || { loans: [] }));
    case FETCH_LOANS_FAILURE:
      return state
        .setIn(['ui', 'getLoansLoading', action.payload.marketID], false)
        .setIn(['ui', 'getLoansError', action.payload.marketID], action.payload.error);

    case DEPOSIT_COLLATERAL_REQUEST:
      return state
        .setIn(['ui', 'depositCollateralLoading'], true)
        .setIn(['ui', 'depositCollateralError'], null);
    case DEPOSIT_COLLATERAL_SUCCESS:
      return state.setIn(['ui', 'depositCollateralLoading'], false);
    case DEPOSIT_COLLATERAL_FAILURE:
      return state
        .setIn(['ui', 'depositCollateralLoading'], false)
        .setIn(['ui', 'depositCollateralError'], action.payload.error);

    case WITHDRAW_COLLATERAL_REQUEST:
      return state
        .setIn(['ui', 'withdrawCollateralLoading'], true)
        .setIn(['ui', 'withdrawCollateralError'], null);
    case WITHDRAW_COLLATERAL_SUCCESS:
      return state.setIn(['ui', 'withdrawCollateralLoading'], false);
    case WITHDRAW_COLLATERAL_FAILURE:
      return state
        .setIn(['ui', 'withdrawCollateralLoading'], false)
        .setIn(['ui', 'withdrawCollateralError'], action.payload.error);

    case BORROW_LOAN_REQUEST:
      return state
        .setIn(['ui', 'borrowLoanLoading'], true)
        .setIn(['ui', 'borrowLoanError'], null);
    case BORROW_LOAN_SUCCESS:
      return state.setIn(['ui', 'borrowLoanLoading'], false);
    case BORROW_LOAN_FAILURE:
      return state
        .setIn(['ui', 'borrowLoanLoading'], false)
        .setIn(['ui', 'borrowLoanError'], action.payload.error);

    case REPAY_LOAN_REQUEST:
      return state
        .setIn(['ui', 'repayLoanLoading'], true)
        .setIn(['ui', 'repayLoanError'], null);
    case REPAY_LOAN_SUCCESS:
      return state.setIn(['ui', 'repayLoanLoading'], false);
    case REPAY_LOAN_FAILURE:
      return state
        .setIn(['ui', 'repayLoanLoading'], false)
        .setIn(['ui', 'repayLoanError'], action.payload.error);

    // Cases for WebSocket updates
    case HANDLE_MARGIN_ACCOUNT_UPDATE:
      if (action.payload && action.payload.marketID) {
        // Assuming payload is the full new details for that market for the current user.
        // This will overwrite existing details for that market.
        // If payload also contains loan updates, they should be part of this payload
        // or handled by a separate WebSocket message type.
        // For example, if action.payload also has a 'loans' field:
        // let newState = state.mergeIn(['accountDetailsByMarket', action.payload.marketID], fromJS(action.payload));
        // if (action.payload.loans) { // Assuming loans are nested under the account update for that market
        //    newState = newState.setIn(['loansByMarket', action.payload.marketID, 'loans'], fromJS(action.payload.loans));
        // }
        // return newState;
        // For now, just updating accountDetailsByMarket based on the prompt
        return state.mergeIn(['accountDetailsByMarket', action.payload.marketID], fromJS(action.payload));
      }
      return state;

    case HANDLE_MARGIN_ALERT:
      // Pushes a new alert object (already given an ID in action creator) to the list
      return state.updateIn(['ui', 'activeMarginAlerts'], alerts => alerts.push(fromJS(action.payload)));

    case DISMISS_MARGIN_ALERT:
      // Filters out the alert with the given ID
      return state.updateIn(['ui', 'activeMarginAlerts'], alerts =>
        alerts.filter(alert => alert.get('id') !== action.payload.alertId)
      );

    case FETCH_SPENDABLE_MARGIN_BALANCE_REQUEST:
      return state
        .setIn(['ui', 'fetchSpendableMarginBalanceLoading', action.payload.marketID, action.payload.assetSymbol], true)
        .setIn(['ui', 'fetchSpendableMarginBalanceError', action.payload.marketID, action.payload.assetSymbol], null);
    case FETCH_SPENDABLE_MARGIN_BALANCE_SUCCESS:
      return state
        .setIn(['ui', 'fetchSpendableMarginBalanceLoading', action.payload.marketID, action.payload.assetSymbol], false)
        .setIn(['marginSpendableBalances', action.payload.marketID, action.payload.assetSymbol], action.payload.amount);
    case FETCH_SPENDABLE_MARGIN_BALANCE_FAILURE:
      return state
        .setIn(['ui', 'fetchSpendableMarginBalanceLoading', action.payload.marketID, action.payload.assetSymbol], false)
        .setIn(['ui', 'fetchSpendableMarginBalanceError', action.payload.marketID, action.payload.assetSymbol], action.payload.error);

    // case HANDLE_AUCTION_UPDATE: // Future use
    //   if (action.payload && action.payload.auctionID) {
    //     if (!state.has('auctions')) { state = state.set('auctions', fromJS({})); }
    //     return state.setIn(['auctions', action.payload.auctionID], fromJS(action.payload));
    //   }
    //   return state;

    default:
      return state;
  }
}
