import { fromJS, List } from 'immutable';
import {
  SELECT_ACCOUNT_TYPE,
  FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST,
  FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS,
  FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE,
  DEPOSIT_COLLATERAL_REQUEST, // Assuming these are handled for UI, though not explicitly part of this subtask's focus
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
  FETCH_OPEN_POSITIONS_REQUEST,
  FETCH_OPEN_POSITIONS_SUCCESS,
  FETCH_OPEN_POSITIONS_FAILURE,
  OPEN_MARGIN_POSITION_REQUEST,
  OPEN_MARGIN_POSITION_UNSIGNED_TX_RECEIVED,
  OPEN_MARGIN_POSITION_FAILURE,
  SIGNING_MARGIN_TRANSACTION_PENDING,
  SIGNING_MARGIN_TRANSACTION_COMPLETE,
  BROADCAST_MARGIN_TRANSACTION_REQUEST,
  BROADCAST_MARGIN_TRANSACTION_SUCCESS,
  BROADCAST_MARGIN_TRANSACTION_FAILURE,
  INITIATE_CLOSE_MARGIN_POSITION_REQUEST,
  INITIATE_CLOSE_MARGIN_POSITION_UNSIGNED_TX_RECEIVED,
  INITIATE_CLOSE_MARGIN_POSITION_FAILURE,
  INITIATE_REPAY_LOAN_REQUEST,
  INITIATE_REPAY_LOAN_UNSIGNED_TX_RECEIVED,
  INITIATE_REPAY_LOAN_FAILURE,
  DEPOSIT_COLLATERAL_UNSIGNED_TX_RECEIVED, // Ensure these are imported
  WITHDRAW_COLLATERAL_UNSIGNED_TX_RECEIVED,
  BORROW_LOAN_UNSIGNED_TX_RECEIVED,
  HANDLE_MARGIN_ACCOUNT_UPDATE,
  HANDLE_MARGIN_ALERT,
  DISMISS_MARGIN_ALERT
} from '../actions/marginActions';

const initialState = fromJS({
  accountDetailsByMarket: {},
  loansByMarket: {},
  openPositions: {
    list: [],
    isLoading: false,
    error: null,
  },
  marginSpendableBalances: {},
  unsignedMarginTx: null,
  lastMarginTxHash: null,
  ui: {
    selectedAccountType: 'spot',
    getMarginAccountDetailsLoading: {},
    getMarginAccountDetailsError: {},
    getLoansLoading: {}, // This might be a map by marketID if fetchLoans is market-specific
    getLoansError: {},   // This might be a map by marketID
    fetchSpendableMarginBalanceLoading: {},
    fetchSpendableMarginBalanceError: {},
    depositCollateralLoading: false,
    depositCollateralError: null,
    withdrawCollateralLoading: false,
    withdrawCollateralError: null,
    borrowLoanLoading: false,
    borrowLoanError: null,
    repayLoanLoading: false, // Original direct repay loading flag
    repayLoanError: null,   // Original direct repay error flag

    isOpeningMarginPosition: false,
    isClosingMarginPosition: false,
    isProcessingRepayLoan: false, // New specific flag for repaying loan
    isSigningInWallet: false,
    isBroadcastingMarginTx: false,
    marginActionError: null,

    activeMarginAlerts: List(),
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

    case FETCH_LOANS_REQUEST: // Assuming fetchLoans can be general or market specific
      let loadingPath = action.payload.marketID
        ? ['ui', 'getLoansLoading', action.payload.marketID]
        : ['ui', 'getLoansLoading', 'all']; // Example for a global flag
      let errorPath = action.payload.marketID
        ? ['ui', 'getLoansError', action.payload.marketID]
        : ['ui', 'getLoansError', 'all'];
      return state.setIn(loadingPath, true).setIn(errorPath, null);
    case FETCH_LOANS_SUCCESS: // Stores loans by marketID even if fetched all
      let successLoadingPath = action.payload.marketID
        ? ['ui', 'getLoansLoading', action.payload.marketID]
        : ['ui', 'getLoansLoading', 'all'];
      if (action.payload.marketID) { // Specific market update
        return state
          .setIn(successLoadingPath, false)
          .setIn(['loansByMarket', action.payload.marketID], fromJS(action.payload.loans || [])); // Corrected fallback
      } else { // If it was a fetch for all loans, the payload should be structured accordingly
        // This part depends on how "fetch all loans" structures its payload.
        // For now, assuming it still comes per market or the reducer is updated if payload is a flat list.
        // If payload.loans is a map of marketID -> loansData:
        // return state.setIn(successLoadingPath, false).mergeIn(['loansByMarket'], fromJS(action.payload.loansByMarket));
        console.warn("FETCH_LOANS_SUCCESS without marketID not fully handled for 'all loans' scenario yet.");
        return state.setIn(successLoadingPath, false);
      }
    case FETCH_LOANS_FAILURE:
      let failureLoadingPath = action.payload.marketID
      ? ['ui', 'getLoansLoading', action.payload.marketID]
      : ['ui', 'getLoansLoading', 'all'];
      let failureErrorPath = action.payload.marketID
        ? ['ui', 'getLoansError', action.payload.marketID]
        : ['ui', 'getLoansError', 'all'];
      return state.setIn(failureLoadingPath, false).setIn(failureErrorPath, action.payload.error);


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

    case FETCH_OPEN_POSITIONS_REQUEST:
      return state
        .setIn(['openPositions', 'isLoading'], true)
        .setIn(['openPositions', 'error'], null);
    case FETCH_OPEN_POSITIONS_SUCCESS:
      return state
        .setIn(['openPositions', 'isLoading'], false)
        .setIn(['openPositions', 'list'], fromJS(action.payload.positions || []));
    case FETCH_OPEN_POSITIONS_FAILURE:
      return state
        .setIn(['openPositions', 'isLoading'], false)
        .setIn(['openPositions', 'error'], action.payload.error);

    // Deposit Collateral
    case DEPOSIT_COLLATERAL_REQUEST:
      return state
        .setIn(['ui', 'depositCollateralLoading'], true)
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'marginActionError'], null)
        .setIn(['ui', 'isSigningInWallet'], false);
    case DEPOSIT_COLLATERAL_UNSIGNED_TX_RECEIVED:
      return state
        .setIn(['ui', 'depositCollateralLoading'], false)
        .set('unsignedMarginTx', fromJS(action.payload));
    case DEPOSIT_COLLATERAL_FAILURE:
      return state
        .setIn(['ui', 'depositCollateralLoading'], false)
        .setIn(['ui', 'isSigningInWallet'], false)
        .setIn(['ui', 'marginActionError'], action.payload.error)
        .set('unsignedMarginTx', null);

    // Withdraw Collateral
    case WITHDRAW_COLLATERAL_REQUEST:
      return state
        .setIn(['ui', 'withdrawCollateralLoading'], true)
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'marginActionError'], null)
        .setIn(['ui', 'isSigningInWallet'], false);
    case WITHDRAW_COLLATERAL_UNSIGNED_TX_RECEIVED:
      return state
        .setIn(['ui', 'withdrawCollateralLoading'], false)
        .set('unsignedMarginTx', fromJS(action.payload));
    case WITHDRAW_COLLATERAL_FAILURE:
      return state
        .setIn(['ui', 'withdrawCollateralLoading'], false)
        .setIn(['ui', 'isSigningInWallet'], false)
        .setIn(['ui', 'marginActionError'], action.payload.error)
        .set('unsignedMarginTx', null);

    // Borrow Loan
    case BORROW_LOAN_REQUEST:
      return state
        .setIn(['ui', 'borrowLoanLoading'], true)
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'marginActionError'], null)
        .setIn(['ui', 'isSigningInWallet'], false);
    case BORROW_LOAN_UNSIGNED_TX_RECEIVED:
      return state
        .setIn(['ui', 'borrowLoanLoading'], false)
        .set('unsignedMarginTx', fromJS(action.payload));
    case BORROW_LOAN_FAILURE:
      return state
        .setIn(['ui', 'borrowLoanLoading'], false)
        .setIn(['ui', 'isSigningInWallet'], false)
        .setIn(['ui', 'marginActionError'], action.payload.error)
        .set('unsignedMarginTx', null);

    // Repay Loan (already has INITIATE_REPAY_LOAN_... which is fine)
    case INITIATE_REPAY_LOAN_REQUEST: // Renamed from REPAY_LOAN_REQUEST if it was different
      return state
        .setIn(['ui', 'isProcessingRepayLoan'], true) // Keeps isProcessingRepayLoan
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'marginActionError'], null)
        .setIn(['ui', 'isSigningInWallet'], false);
    case INITIATE_REPAY_LOAN_UNSIGNED_TX_RECEIVED: // Renamed from REPAY_LOAN_UNSIGNED_TX_RECEIVED
      return state
        .setIn(['ui', 'isProcessingRepayLoan'], false)
        .set('unsignedMarginTx', fromJS(action.payload));
    case INITIATE_REPAY_LOAN_FAILURE: // Renamed from REPAY_LOAN_FAILURE
      return state
        .setIn(['ui', 'isProcessingRepayLoan'], false)
        .setIn(['ui', 'isSigningInWallet'], false)
        .setIn(['ui', 'marginActionError'], action.payload.error)
        .set('unsignedMarginTx', null);

    // Open Margin Position
    case OPEN_MARGIN_POSITION_REQUEST:
      return state
        .setIn(['ui', 'isOpeningMarginPosition'], true)
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'marginActionError'], null)
        .setIn(['ui', 'isSigningInWallet'], false);
    case OPEN_MARGIN_POSITION_UNSIGNED_TX_RECEIVED:
      return state
        .setIn(['ui', 'isOpeningMarginPosition'], false)
        .set('unsignedMarginTx', fromJS(action.payload));
    case OPEN_MARGIN_POSITION_FAILURE:
      return state
        .setIn(['ui', 'isOpeningMarginPosition'], false)
        .setIn(['ui', 'isSigningInWallet'], false)
        .setIn(['ui', 'marginActionError'], action.payload.error)
        .set('unsignedMarginTx', null);

    case INITIATE_CLOSE_MARGIN_POSITION_REQUEST:
      return state
        .setIn(['ui', 'isClosingMarginPosition'], true)
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'marginActionError'], null)
        .setIn(['ui', 'isSigningInWallet'], false);
    case INITIATE_CLOSE_MARGIN_POSITION_UNSIGNED_TX_RECEIVED:
      return state
        .setIn(['ui', 'isClosingMarginPosition'], false)
        .set('unsignedMarginTx', fromJS(action.payload));
    case INITIATE_CLOSE_MARGIN_POSITION_FAILURE:
      return state
        .setIn(['ui', 'isClosingMarginPosition'], false)
        .setIn(['ui', 'isSigningInWallet'], false)
        .setIn(['ui', 'marginActionError'], action.payload.error)
        .set('unsignedMarginTx', null);

    case SIGNING_MARGIN_TRANSACTION_PENDING:
      return state
        .setIn(['ui', 'isSigningInWallet'], true)
        .setIn(['ui', 'marginActionError'], null);
    case SIGNING_MARGIN_TRANSACTION_COMPLETE:
      return state.setIn(['ui', 'isSigningInWallet'], false);

    case BROADCAST_MARGIN_TRANSACTION_REQUEST:
      return state
        .setIn(['ui', 'isBroadcastingMarginTx'], true)
        .setIn(['ui', 'isSigningInWallet'], false)
        .setIn(['ui', 'marginActionError'], null);
    case BROADCAST_MARGIN_TRANSACTION_SUCCESS:
      return state
        .setIn(['ui', 'isBroadcastingMarginTx'], false)
        .set('lastMarginTxHash', action.payload.transactionHash)
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'isSigningInWallet'], false) // Ensure signing is also complete
        .setIn(['ui', 'isOpeningMarginPosition'], false)
        .setIn(['ui', 'isClosingMarginPosition'], false)
        .setIn(['ui', 'isProcessingRepayLoan'], false)
        .setIn(['ui', 'depositCollateralLoading'], false)   // Reset these too
        .setIn(['ui', 'withdrawCollateralLoading'], false)
        .setIn(['ui', 'borrowLoanLoading'], false);
    case BROADCAST_MARGIN_TRANSACTION_FAILURE:
      return state
        .setIn(['ui', 'isBroadcastingMarginTx'], false)
        .setIn(['ui', 'marginActionError'], action.payload.error)
        .set('unsignedMarginTx', null)
        .setIn(['ui', 'isSigningInWallet'], false) // Ensure signing is also complete
        .setIn(['ui', 'isOpeningMarginPosition'], false)
        .setIn(['ui', 'isClosingMarginPosition'], false)
        .setIn(['ui', 'isProcessingRepayLoan'], false)
        .setIn(['ui', 'depositCollateralLoading'], false)   // Reset these too
        .setIn(['ui', 'withdrawCollateralLoading'], false)
        .setIn(['ui', 'borrowLoanLoading'], false);

    case HANDLE_MARGIN_ACCOUNT_UPDATE:
      if (action.payload && action.payload.marketID) {
        return state.mergeIn(['accountDetailsByMarket', action.payload.marketID], fromJS(action.payload));
      }
      return state;
    case HANDLE_MARGIN_ALERT:
      return state.updateIn(['ui', 'activeMarginAlerts'], alerts => alerts.push(fromJS(action.payload)));
    case DISMISS_MARGIN_ALERT:
      return state.updateIn(['ui', 'activeMarginAlerts'], alerts =>
        alerts.filter(alert => alert.get('id') !== action.payload.alertId)
      );

    default:
      return state;
  }
}
