import api from '../lib/api'; // Import the API library

// Action Type Constants
export const SELECT_ACCOUNT_TYPE = 'margin/SELECT_ACCOUNT_TYPE';

export const FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST = 'margin/FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST';
export const FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS = 'margin/FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS';
export const FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE = 'margin/FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE';

export const DEPOSIT_COLLATERAL_REQUEST = 'margin/DEPOSIT_COLLATERAL_REQUEST';
export const DEPOSIT_COLLATERAL_SUCCESS = 'margin/DEPOSIT_COLLATERAL_SUCCESS';
export const DEPOSIT_COLLATERAL_FAILURE = 'margin/DEPOSIT_COLLATERAL_FAILURE';

export const WITHDRAW_COLLATERAL_REQUEST = 'margin/WITHDRAW_COLLATERAL_REQUEST';
export const WITHDRAW_COLLATERAL_SUCCESS = 'margin/WITHDRAW_COLLATERAL_SUCCESS';
export const WITHDRAW_COLLATERAL_FAILURE = 'margin/WITHDRAW_COLLATERAL_FAILURE';

export const BORROW_LOAN_REQUEST = 'margin/BORROW_LOAN_REQUEST';
export const BORROW_LOAN_SUCCESS = 'margin/BORROW_LOAN_SUCCESS';
export const BORROW_LOAN_FAILURE = 'margin/BORROW_LOAN_FAILURE';

export const REPAY_LOAN_REQUEST = 'margin/REPAY_LOAN_REQUEST';
export const REPAY_LOAN_SUCCESS = 'margin/REPAY_LOAN_SUCCESS';
export const REPAY_LOAN_FAILURE = 'margin/REPAY_LOAN_FAILURE';

export const FETCH_LOANS_REQUEST = 'margin/FETCH_LOANS_REQUEST';
export const FETCH_LOANS_SUCCESS = 'margin/FETCH_LOANS_SUCCESS';
export const FETCH_LOANS_FAILURE = 'margin/FETCH_LOANS_FAILURE';

// WebSocket related actions
export const HANDLE_MARGIN_ACCOUNT_UPDATE = 'margin/HANDLE_MARGIN_ACCOUNT_UPDATE';
export const HANDLE_MARGIN_ALERT = 'margin/HANDLE_MARGIN_ALERT';
export const DISMISS_MARGIN_ALERT = 'margin/DISMISS_MARGIN_ALERT';
// export const HANDLE_AUCTION_UPDATE = 'margin/HANDLE_AUCTION_UPDATE'; // Future use

// Action Creators

export const selectAccountType = (accountType) => ({
  type: SELECT_ACCOUNT_TYPE,
  payload: { accountType }
});

export const fetchMarginAccountDetails = (marketID, userAddress) => async (dispatch) => {
  dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST, payload: { marketID, userAddress } });
  try {
    const response = await api.getMarginAccountDetails(marketID, userAddress);
    // TODO: Check response.data.status for backend-specific error codes (e.g., if response.data.status !== 0)
    if (response && response.data && response.data.data !== undefined) { // Assuming successful response has a 'data' field within response.data
      dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS, payload: { marketID, userAddress, details: response.data.data } });
      return response.data.data;
    } else {
      // Handle cases where response is not as expected but not a network error
      const errorMsg = response && response.data && response.data.desc ? response.data.desc : 'Invalid response structure';
      dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE, payload: { marketID, userAddress, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE, payload: { marketID, userAddress, error: error.message } });
    throw error; // Re-throw to allow calling component to handle
  }
};

export const depositCollateral = (marketID, assetAddress, amount) => async (dispatch) => {
  dispatch({ type: DEPOSIT_COLLATERAL_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.depositToCollateral({ marketID, assetAddress, amount: amount.toString() });
    // TODO: Check response.data.status for backend-specific error codes
    if (response && response.data && response.data.data !== undefined) {
      dispatch({ type: DEPOSIT_COLLATERAL_SUCCESS, payload: { marketID, assetAddress, amount, data: response.data.data } });
      // TODO: Consider re-fetching margin account details or loans after success
      // dispatch(fetchMarginAccountDetails(marketID, userAddress)); // userAddress would be needed here
      // dispatch(fetchLoans(marketID, userAddress));
      return response.data.data;
    } else {
      const errorMsg = response && response.data && response.data.desc ? response.data.desc : 'Invalid response structure';
      dispatch({ type: DEPOSIT_COLLATERAL_FAILURE, payload: { marketID, assetAddress, amount, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: DEPOSIT_COLLATERAL_FAILURE, payload: { marketID, assetAddress, amount, error: error.message } });
    throw error;
  }
};

// Action creators for WebSocket messages
export const handleMarginAccountUpdate = (updateData) => ({
  type: HANDLE_MARGIN_ACCOUNT_UPDATE,
  payload: updateData // Expected: { marketID, userAddress (optional), ...details }
});

export const handleMarginAlert = (alertData) => ({
  type: HANDLE_MARGIN_ALERT,
  payload: { ...alertData, id: Date.now() } // Add a unique ID for dismissability
});

export const dismissMarginAlert = (alertId) => ({
  type: DISMISS_MARGIN_ALERT,
  payload: { alertId }
});

// export const handleAuctionUpdate = (auctionData) => ({
//   type: HANDLE_AUCTION_UPDATE,
//   payload: auctionData
// });

export const withdrawCollateral = (marketID, assetAddress, amount) => async (dispatch) => {
  dispatch({ type: WITHDRAW_COLLATERAL_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.withdrawFromCollateral({ marketID, assetAddress, amount: amount.toString() });
    if (response && response.data && response.data.data !== undefined) {
      dispatch({ type: WITHDRAW_COLLATERAL_SUCCESS, payload: { marketID, assetAddress, amount, data: response.data.data } });
      // TODO: Consider re-fetching margin account details or loans
      return response.data.data;
    } else {
      const errorMsg = response && response.data && response.data.desc ? response.data.desc : 'Invalid response structure';
      dispatch({ type: WITHDRAW_COLLATERAL_FAILURE, payload: { marketID, assetAddress, amount, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: WITHDRAW_COLLATERAL_FAILURE, payload: { marketID, assetAddress, amount, error: error.message } });
    throw error;
  }
};

export const borrowLoanAction = (marketID, assetAddress, amount) => async (dispatch) => {
  dispatch({ type: BORROW_LOAN_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.borrowLoan({ marketID, assetAddress, amount: amount.toString() });
    if (response && response.data && response.data.data !== undefined) {
      dispatch({ type: BORROW_LOAN_SUCCESS, payload: { marketID, assetAddress, amount, data: response.data.data } });
      // TODO: Consider re-fetching margin account details and loans
      return response.data.data;
    } else {
      const errorMsg = response && response.data && response.data.desc ? response.data.desc : 'Invalid response structure';
      dispatch({ type: BORROW_LOAN_FAILURE, payload: { marketID, assetAddress, amount, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: BORROW_LOAN_FAILURE, payload: { marketID, assetAddress, amount, error: error.message } });
    throw error;
  }
};

export const repayLoanAction = (marketID, assetAddress, amount) => async (dispatch) => {
  dispatch({ type: REPAY_LOAN_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.repayLoan({ marketID, assetAddress, amount: amount.toString() });
    if (response && response.data && response.data.data !== undefined) {
      dispatch({ type: REPAY_LOAN_SUCCESS, payload: { marketID, assetAddress, amount, data: response.data.data } });
      // TODO: Consider re-fetching margin account details and loans
      return response.data.data;
    } else {
      const errorMsg = response && response.data && response.data.desc ? response.data.desc : 'Invalid response structure';
      dispatch({ type: REPAY_LOAN_FAILURE, payload: { marketID, assetAddress, amount, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: REPAY_LOAN_FAILURE, payload: { marketID, assetAddress, amount, error: error.message } });
    throw error;
  }
};

export const fetchLoans = (marketID, userAddress) => async (dispatch) => {
  dispatch({ type: FETCH_LOANS_REQUEST, payload: { marketID, userAddress } });
  try {
    const response = await api.getLoans(marketID, userAddress);
    if (response && response.data && response.data.data !== undefined) {
      dispatch({ type: FETCH_LOANS_SUCCESS, payload: { marketID, userAddress, loans: response.data.data } }); // Changed 'data' to 'loans' for clarity
      return response.data.data;
    } else {
      const errorMsg = response && response.data && response.data.desc ? response.data.desc : 'Invalid response structure';
      dispatch({ type: FETCH_LOANS_FAILURE, payload: { marketID, userAddress, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: FETCH_LOANS_FAILURE, payload: { marketID, userAddress, error: error.message } });
    throw error;
  }
};
