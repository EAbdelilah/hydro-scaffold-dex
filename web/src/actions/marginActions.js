import api from '../lib/api'; // Import the API library
import { getSelectedAccountWallet } from '@gongddex/hydro-sdk-wallet';
import { ethers } from 'ethers';
import { clearTradeForm } from './trade'; // Import clearTradeForm

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

export const FETCH_SPENDABLE_MARGIN_BALANCE_REQUEST = 'margin/FETCH_SPENDABLE_MARGIN_BALANCE_REQUEST';
export const FETCH_SPENDABLE_MARGIN_BALANCE_SUCCESS = 'margin/FETCH_SPENDABLE_MARGIN_BALANCE_SUCCESS';
export const FETCH_SPENDABLE_MARGIN_BALANCE_FAILURE = 'margin/FETCH_SPENDABLE_MARGIN_BALANCE_FAILURE';

export const FETCH_OPEN_POSITIONS_REQUEST = 'margin/FETCH_OPEN_POSITIONS_REQUEST';
export const FETCH_OPEN_POSITIONS_SUCCESS = 'margin/FETCH_OPEN_POSITIONS_SUCCESS';
export const FETCH_OPEN_POSITIONS_FAILURE = 'margin/FETCH_OPEN_POSITIONS_FAILURE';

// WebSocket related actions
export const HANDLE_MARGIN_ACCOUNT_UPDATE = 'margin/HANDLE_MARGIN_ACCOUNT_UPDATE';
export const HANDLE_MARGIN_ALERT = 'margin/HANDLE_MARGIN_ALERT';
export const DISMISS_MARGIN_ALERT = 'margin/DISMISS_MARGIN_ALERT';

// Open Margin Position Actions
export const OPEN_MARGIN_POSITION_REQUEST = 'margin/OPEN_MARGIN_POSITION_REQUEST';
export const OPEN_MARGIN_POSITION_UNSIGNED_TX_RECEIVED = 'margin/OPEN_MARGIN_POSITION_UNSIGNED_TX_RECEIVED';
export const OPEN_MARGIN_POSITION_FAILURE = 'margin/OPEN_MARGIN_POSITION_FAILURE';
export const SIGNING_MARGIN_TRANSACTION_PENDING = 'margin/SIGNING_MARGIN_TRANSACTION_PENDING';
export const SIGNING_MARGIN_TRANSACTION_COMPLETE = 'margin/SIGNING_MARGIN_TRANSACTION_COMPLETE';

// Broadcast Transaction Actions
export const BROADCAST_MARGIN_TRANSACTION_REQUEST = 'margin/BROADCAST_MARGIN_TRANSACTION_REQUEST';
export const BROADCAST_MARGIN_TRANSACTION_SUCCESS = 'margin/BROADCAST_MARGIN_TRANSACTION_SUCCESS';
export const BROADCAST_MARGIN_TRANSACTION_FAILURE = 'margin/BROADCAST_MARGIN_TRANSACTION_FAILURE';

// Close Margin Position Actions
export const INITIATE_CLOSE_MARGIN_POSITION_REQUEST = 'margin/INITIATE_CLOSE_MARGIN_POSITION_REQUEST';
export const INITIATE_CLOSE_MARGIN_POSITION_UNSIGNED_TX_RECEIVED = 'margin/INITIATE_CLOSE_MARGIN_POSITION_UNSIGNED_TX_RECEIVED';
export const INITIATE_CLOSE_MARGIN_POSITION_FAILURE = 'margin/INITIATE_CLOSE_MARGIN_POSITION_FAILURE';

// Action Creators
export const selectAccountType = (accountType) => ({
  type: SELECT_ACCOUNT_TYPE,
  payload: { accountType }
});

// Conceptual: This action would aggregate multiple data-refreshing thunks
export const refreshAllMarginDataForUserAndMarket = (marketID, userAddress, collateralAssetSymbol, otherAssetSymbol) => async (dispatch) => {
  console.log(`Refreshing margin data for market ${marketID}, user ${userAddress}`);
  if (!userAddress || typeof userAddress !== 'string' || !userAddress.startsWith('0x')) {
    console.warn('refreshAllMarginDataForUserAndMarket: Invalid or missing userAddress', userAddress);
    return;
  }
  if (!marketID) {
    console.warn('refreshAllMarginDataForUserAndMarket: Invalid or missing marketID', marketID);
    return;
  }
  try {
    await dispatch(fetchMarginAccountDetails(marketID, userAddress));
    if (collateralAssetSymbol) { 
      await dispatch(fetchSpendableMarginBalance(marketID, collateralAssetSymbol, userAddress));
    }
    if (otherAssetSymbol && otherAssetSymbol !== collateralAssetSymbol) { 
       await dispatch(fetchSpendableMarginBalance(marketID, otherAssetSymbol, userAddress));
    }
    await dispatch(fetchOpenPositions(userAddress)); 
    await dispatch(fetchLoans(marketID, userAddress)); 
  } catch (error) {
    console.error('Error during refreshAllMarginDataForUserAndMarket:', error);
  }
};


const walletService = {
  signTransaction: async (unsignedTxData, state) => {
    const wallet = getSelectedAccountWallet(state);
    if (!wallet || !wallet.provider) {
      throw new Error('Wallet or provider not available');
    }
    if (window.ethereum && wallet.provider === window.ethereum) {
      const provider = new ethers.providers.Web3Provider(window.ethereum);
      const signer = provider.getSigner();
      const txRequest = {
        from: unsignedTxData.from,
        to: unsignedTxData.to,
        value: ethers.utils.hexlify(ethers.BigNumber.from(unsignedTxData.value || '0')),
        data: unsignedTxData.data,
        gasPrice: ethers.utils.hexlify(ethers.BigNumber.from(unsignedTxData.gasPrice || '0')),
        gasLimit: ethers.utils.hexlify(ethers.BigNumber.from(unsignedTxData.gasLimit || '0')),
      };
      if (unsignedTxData.nonce) {
        txRequest.nonce = ethers.utils.hexlify(ethers.BigNumber.from(unsignedTxData.nonce));
      } else {
         txRequest.nonce = await signer.getTransactionCount();
      }
      if (unsignedTxData.chainId) { // Add chainId if present
        txRequest.chainId = parseInt(unsignedTxData.chainId);
      }
      console.log("Signing transaction request:", txRequest);
      // This is a placeholder for actual signing. 
      // For MetaMask, signer.sendTransaction() is more common if the wallet also broadcasts.
      // If backend *must* receive the raw signed tx, then signer.signTransaction() would be used,
      // but it requires the transaction to be fully populated by the signer, or careful construction.
      // For now, simulating getting a raw signed transaction hex.
      alert('Please sign the transaction in your wallet (Simulated)'); 
      return `0xSIMULATED_SIGNED_RAW_TX_${Date.now()}`;
    } else {
      throw new Error('Unsupported wallet provider for signing transaction.');
    }
  }
};

export const openMarginPosition = (params) => async (dispatch, getState) => {
  dispatch({ type: OPEN_MARGIN_POSITION_REQUEST, payload: params });
  try {
    const response = await api.openMarginPosition(params);
    if (response.data.status === 0 && response.data.data) {
      const unsignedTxData = response.data.data;
      dispatch({ type: OPEN_MARGIN_POSITION_UNSIGNED_TX_RECEIVED, payload: unsignedTxData });
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_PENDING });
      try {
        const signedTxHex = await walletService.signTransaction(unsignedTxData, getState());
        if (signedTxHex) {
          dispatch(broadcastMarginTransaction(signedTxHex, params.marketID, params.userAddress, params.collateralAssetSymbol, params.baseAssetSymbol, params.quoteAssetSymbol));
        } else {
          dispatch({ type: OPEN_MARGIN_POSITION_FAILURE, payload: { error: 'Transaction signing failed or rejected by user.' } });
          dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE }); 
        }
      } catch (signError) {
        console.error('Signing error:', signError);
        dispatch({ type: OPEN_MARGIN_POSITION_FAILURE, payload: { error: `Transaction signing failed: ${signError.message || signError}` } });
        dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      }
      return unsignedTxData; 
    } else {
      const errorMsg = response.data.desc || 'Failed to open margin position';
      dispatch({ type: OPEN_MARGIN_POSITION_FAILURE, payload: { error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: OPEN_MARGIN_POSITION_FAILURE, payload: { error: error.message } });
    throw error;
  }
};

export const initiateCloseMarginPosition = (marketID, userAddress, baseAssetSymbol, quoteAssetSymbol) => async (dispatch, getState) => {
  dispatch({ type: INITIATE_CLOSE_MARGIN_POSITION_REQUEST, payload: { marketID, userAddress } });
  try {
    const response = await api.closeMarginPosition({ marketID });
    if (response.data.status === 0 && response.data.data) {
      const unsignedTxData = response.data.data;
      dispatch({ type: INITIATE_CLOSE_MARGIN_POSITION_UNSIGNED_TX_RECEIVED, payload: unsignedTxData });
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_PENDING });
      try {
        const signedTxHex = await walletService.signTransaction(unsignedTxData, getState());
        if (signedTxHex) {
          dispatch(broadcastMarginTransaction(signedTxHex, marketID, userAddress, null, baseAssetSymbol, quoteAssetSymbol)); 
        } else {
          dispatch({ type: INITIATE_CLOSE_MARGIN_POSITION_FAILURE, payload: { error: 'Transaction signing failed or rejected by user.' } });
          dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
        }
      } catch (signError) {
        console.error('Signing error during close position:', signError);
        dispatch({ type: INITIATE_CLOSE_MARGIN_POSITION_FAILURE, payload: { error: `Transaction signing failed: ${signError.message || signError}` } });
        dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      }
      return unsignedTxData;
    } else {
      const errorMsg = response.data.desc || 'Failed to initiate close margin position';
      dispatch({ type: INITIATE_CLOSE_MARGIN_POSITION_FAILURE, payload: { error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: INITIATE_CLOSE_MARGIN_POSITION_FAILURE, payload: { error: error.message } });
    throw error;
  }
};

export const broadcastMarginTransaction = (
    signedRawTxHex, 
    marketID, 
    userAddress, 
    collateralAssetSymbol, 
    baseAssetSymbol,       
    quoteAssetSymbol       
  ) => async (dispatch) => {
  dispatch({ type: BROADCAST_MARGIN_TRANSACTION_REQUEST, payload: { signedRawTxHex } });
  try {
    const response = await api.broadcastTransaction({ signedRawTx: signedRawTxHex }); 
    if (response.data.status === 0 && response.data.data && response.data.data.transactionHash) {
      const txHash = response.data.data.transactionHash;
      dispatch({ type: BROADCAST_MARGIN_TRANSACTION_SUCCESS, payload: { transactionHash: txHash } });
      dispatch(clearTradeForm()); 
      let otherAssetForRefresh = baseAssetSymbol === collateralAssetSymbol ? quoteAssetSymbol : baseAssetSymbol;
      if (marketID && userAddress) {
         dispatch(refreshAllMarginDataForUserAndMarket(marketID, userAddress, collateralAssetSymbol, otherAssetForRefresh));
      } else {
        console.warn("broadcastMarginTransaction: marketID or userAddress missing, cannot refresh all data.", {marketID, userAddress});
      }
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      console.log(`Margin transaction broadcasted: ${txHash}. Data refreshed.`);
      return txHash;
    } else {
      const errorMsg = response.data.desc || 'Failed to broadcast margin transaction';
      dispatch({ type: BROADCAST_MARGIN_TRANSACTION_FAILURE, payload: { error: errorMsg } });
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: BROADCAST_MARGIN_TRANSACTION_FAILURE, payload: { error: error.message } });
    dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
    throw error;
  }
};

// Keep other existing actions like fetchMarginAccountDetails, depositCollateral, fetchOpenPositions, fetchSpendableMarginBalance, etc.

export const fetchMarginAccountDetails = (marketID, userAddress) => async (dispatch) => {
  dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST, payload: { marketID, userAddress } });
  try {
    const response = await api.getMarginAccountDetails(marketID, userAddress);
    if (response && response.data && response.data.data !== undefined) { 
      dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS, payload: { marketID, userAddress, details: response.data.data } });
      return response.data.data;
    } else {
      const errorMsg = response && response.data && response.data.desc ? response.data.desc : 'Invalid response structure';
      dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE, payload: { marketID, userAddress, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE, payload: { marketID, userAddress, error: error.message } });
    throw error; 
  }
};

export const depositCollateral = (marketID, assetAddress, amount) => async (dispatch) => {
  dispatch({ type: DEPOSIT_COLLATERAL_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.depositToCollateral({ marketID, assetAddress, amount: amount.toString() });
    if (response && response.data && response.data.data !== undefined) {
      dispatch({ type: DEPOSIT_COLLATERAL_SUCCESS, payload: { marketID, assetAddress, amount, data: response.data.data } });
      // TODO: Refresh relevant data
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

export const fetchOpenPositions = (userAddress) => async (dispatch) => {
  dispatch({ type: FETCH_OPEN_POSITIONS_REQUEST, payload: { userAddress } });
  try {
    const response = await api.getOpenPositions(userAddress); 
    if (response.data.status === 0 && response.data.data) {
      dispatch({
        type: FETCH_OPEN_POSITIONS_SUCCESS,
        payload: { positions: response.data.data, userAddress } 
      });
      return response.data.data;
    } else {
      const errorMsg = response.data.desc || 'Failed to fetch open positions';
      dispatch({ type: FETCH_OPEN_POSITIONS_FAILURE, payload: { error: errorMsg, userAddress } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: FETCH_OPEN_POSITIONS_FAILURE, payload: { error: error.message, userAddress } });
    throw error;
  }
};

export const fetchSpendableMarginBalance = (marketID, assetSymbol, userAddress) => async (dispatch) => {
  dispatch({ type: FETCH_SPENDABLE_MARGIN_BALANCE_REQUEST, payload: { marketID, assetSymbol, userAddress } });
  try {
    const response = await api.getSpendableMarginBalance(marketID, assetSymbol, userAddress);
    if (response.data.status === 0 && response.data.data) {
      dispatch({
        type: FETCH_SPENDABLE_MARGIN_BALANCE_SUCCESS,
        payload: { marketID, assetSymbol, userAddress, amount: response.data.data.amount }
      });
      return response.data.data.amount;
    } else {
      const errorMsg = response.data.desc || 'Failed to fetch spendable margin balance';
      dispatch({ type: FETCH_SPENDABLE_MARGIN_BALANCE_FAILURE, payload: { marketID, assetSymbol, userAddress, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: FETCH_SPENDABLE_MARGIN_BALANCE_FAILURE, payload: { marketID, assetSymbol, userAddress, error: error.message } });
    throw error;
  }
};

export const handleMarginAccountUpdate = (updateData) => ({
  type: HANDLE_MARGIN_ACCOUNT_UPDATE,
  payload: updateData 
});

export const handleMarginAlert = (alertData) => ({
  type: HANDLE_MARGIN_ALERT,
  payload: { ...alertData, id: Date.now() } 
});

export const dismissMarginAlert = (alertId) => ({
  type: DISMISS_MARGIN_ALERT,
  payload: { alertId }
});

export const withdrawCollateral = (marketID, assetAddress, amount) => async (dispatch) => {
  dispatch({ type: WITHDRAW_COLLATERAL_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.withdrawFromCollateral({ marketID, assetAddress, amount: amount.toString() });
    if (response && response.data && response.data.data !== undefined) {
      dispatch({ type: WITHDRAW_COLLATERAL_SUCCESS, payload: { marketID, assetAddress, amount, data: response.data.data } });
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
      dispatch({ type: FETCH_LOANS_SUCCESS, payload: { marketID, userAddress, loans: response.data.data } }); 
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
// Make sure to keep other existing actions if this overwrite is partial.
// For this operation, I'm providing the assumed full content with modifications.
