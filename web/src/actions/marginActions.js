import api from '../lib/api'; // Import the API library
import { getSelectedAccountWallet } from '@gongddex/hydro-sdk-wallet';
import { ethers } from 'ethers';
import { clearTradeForm } from './trade'; // Import clearTradeForm
import { showMarginAlert } from './notificationActions'; // Import for success notification

// Action Type Constants
export const SELECT_ACCOUNT_TYPE = 'margin/SELECT_ACCOUNT_TYPE';

export const FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST = 'margin/FETCH_MARGIN_ACCOUNT_DETAILS_REQUEST';
export const FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS = 'margin/FETCH_MARGIN_ACCOUNT_DETAILS_SUCCESS';
export const FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE = 'margin/FETCH_MARGIN_ACCOUNT_DETAILS_FAILURE';

export const DEPOSIT_COLLATERAL_REQUEST = 'margin/DEPOSIT_COLLATERAL_REQUEST';
export const DEPOSIT_COLLATERAL_UNSIGNED_TX_RECEIVED = 'margin/DEPOSIT_COLLATERAL_UNSIGNED_TX_RECEIVED';
export const DEPOSIT_COLLATERAL_SUCCESS = 'margin/DEPOSIT_COLLATERAL_SUCCESS'; // Kept for optimistic updates or direct calls if ever needed
export const DEPOSIT_COLLATERAL_FAILURE = 'margin/DEPOSIT_COLLATERAL_FAILURE';

export const WITHDRAW_COLLATERAL_REQUEST = 'margin/WITHDRAW_COLLATERAL_REQUEST';
export const WITHDRAW_COLLATERAL_UNSIGNED_TX_RECEIVED = 'margin/WITHDRAW_COLLATERAL_UNSIGNED_TX_RECEIVED';
export const WITHDRAW_COLLATERAL_SUCCESS = 'margin/WITHDRAW_COLLATERAL_SUCCESS';
export const WITHDRAW_COLLATERAL_FAILURE = 'margin/WITHDRAW_COLLATERAL_FAILURE';

export const BORROW_LOAN_REQUEST = 'margin/BORROW_LOAN_REQUEST';
export const BORROW_LOAN_UNSIGNED_TX_RECEIVED = 'margin/BORROW_LOAN_UNSIGNED_TX_RECEIVED';
export const BORROW_LOAN_SUCCESS = 'margin/BORROW_LOAN_SUCCESS';
export const BORROW_LOAN_FAILURE = 'margin/BORROW_LOAN_FAILURE';

export const REPAY_LOAN_REQUEST = 'margin/REPAY_LOAN_REQUEST';
export const REPAY_LOAN_UNSIGNED_TX_RECEIVED = 'margin/REPAY_LOAN_UNSIGNED_TX_RECEIVED';
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
  // signTransaction: async (unsignedTxData, getState) => { // getState removed
  signTransaction: async (unsignedTxData) => {
    console.log("walletService.signTransaction: Received unsignedTxData:", unsignedTxData);

    if (!window.ethereum) {
        throw new Error("Ethereum wallet (e.g., MetaMask) not detected. Please install or enable your wallet.");
    }

    try {
        // Optional: Dynamically request accounts if not already connected
        // await window.ethereum.request({ method: 'eth_requestAccounts' });

        const provider = new ethers.providers.Web3Provider(window.ethereum);
        const signer = provider.getSigner();
        const signerAddress = await signer.getAddress();

        // Strict check: unsignedTxData.from MUST match the signer's address for this flow.
        if (unsignedTxData.from && signerAddress.toLowerCase() !== unsignedTxData.from.toLowerCase()) {
            console.error(`Wallet address mismatch. Expected: ${unsignedTxData.from}, Wallet: ${signerAddress}.`);
            throw new Error(`Wallet address mismatch. Expected ${unsignedTxData.from}, but selected account is ${signerAddress}. Please switch accounts in your wallet or ensure 'from' field is correct.`);
        }

        const txRequest = {
            to: unsignedTxData.to,
            // from: unsignedTxData.from, // Let ethers.js populate 'from' via signer.populateTransaction
            nonce: ethers.BigNumber.from(unsignedTxData.nonce).toNumber(),
            gasLimit: ethers.BigNumber.from(unsignedTxData.gasLimit),
            gasPrice: ethers.BigNumber.from(unsignedTxData.gasPrice),
            data: unsignedTxData.data,
            value: ethers.BigNumber.from(unsignedTxData.value || '0'), // Keep default to '0'
            chainId: parseInt(unsignedTxData.chainId)
        };

        console.log("walletService.signTransaction: Populating transaction:", txRequest);
        const populatedTx = await signer.populateTransaction(txRequest);

        // Remove 'from' if populated, as signTransaction often derives it from the signer itself
        // delete populatedTx.from; // This can be necessary for some wallets/signers

        console.log("walletService.signTransaction: Requesting signature for populated TX:", populatedTx);
        const signedRawTxHex = await signer.signTransaction(populatedTx);

        console.info("Transaction signed successfully:", signedRawTxHex);
        return signedRawTxHex;

    } catch (error) {
        console.error("walletService.signTransaction: Error during signing:", error);
        if (error.code === 4001 || error.code === 'ACTION_REJECTED') { // MetaMask user rejection or Ethers v5 rejection
            throw new Error("Transaction signature rejected by user.");
        }
        const mainReason = error.reason || (error.data ? error.data.message : (error.error ? error.error.message : error.message));
        throw new Error(mainReason || "Failed to sign transaction with wallet.");
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
        const signedTxHex = await walletService.signTransaction(unsignedTxData); // getState removed
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
        const signedTxHex = await walletService.signTransaction(unsignedTxData); // getState removed
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

      // Dispatch success notification
      dispatch(showMarginAlert({
        level: 'success',
        message: `Transaction submitted: ${txHash.substring(0, 10)}...`, // Shorten for display
        txHash: txHash,
        autoDismiss: 5000
      }));

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
      // Dispatch error notification for broadcast failure
      dispatch(showMarginAlert({ level: 'error', message: `Transaction broadcast failed: ${errorMsg}` }));
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: BROADCAST_MARGIN_TRANSACTION_FAILURE, payload: { error: error.message } });
    // Dispatch error notification for broadcast failure (network error, etc.)
    dispatch(showMarginAlert({ level: 'error', message: `Transaction broadcast failed: ${error.message}` }));
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

export const depositCollateral = (marketID, userAddress, assetAddress, assetSymbol, amount) => async (dispatch, getState) => {
  dispatch({ type: DEPOSIT_COLLATERAL_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    // Assuming api.depositToCollateral now returns unsignedTxData
    const response = await api.depositToCollateral({ marketID, assetAddress, amount: amount.toString() });
    if (response.data.status === 0 && response.data.data) {
      const unsignedTxData = response.data.data;
      dispatch({ type: DEPOSIT_COLLATERAL_UNSIGNED_TX_RECEIVED, payload: unsignedTxData });
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_PENDING });
      try {
        const signedTxHex = await walletService.signTransaction(unsignedTxData); // getState removed
        if (signedTxHex) {
          const market = getState().market.markets[marketID]; // getState still needed here for market details
          const baseAssetSymbol = market ? market.baseTokenSymbol : null;
          const quoteAssetSymbol = market ? market.quoteTokenSymbol : null;
          dispatch(broadcastMarginTransaction(signedTxHex, marketID, userAddress, assetSymbol, baseAssetSymbol, quoteAssetSymbol));
        } else {
          dispatch({ type: DEPOSIT_COLLATERAL_FAILURE, payload: { error: 'Transaction signing failed or rejected by user.' } });
          dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
        }
      } catch (signError) {
        console.error('Signing error during deposit collateral:', signError);
        dispatch({ type: DEPOSIT_COLLATERAL_FAILURE, payload: { error: `Transaction signing failed: ${signError.message || signError}` } });
        dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      }
      return unsignedTxData;
    } else {
      const errorMsg = response.data.desc || 'Failed to initiate deposit collateral';
      dispatch({ type: DEPOSIT_COLLATERAL_FAILURE, payload: { error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: DEPOSIT_COLLATERAL_FAILURE, payload: { error: error.message } });
    throw error;
  }
};

export const fetchOpenPositions = (userAddress) => async (dispatch, getState) => { // userAddress is still useful for storing data in redux state by user
  // If userAddress is not passed, try to get it from the selected account in wallet state
  const effectiveUserAddress = userAddress || (getSelectedAccountWallet(getState())?.address);
  if (!effectiveUserAddress) {
    dispatch({ type: FETCH_OPEN_POSITIONS_FAILURE, payload: { error: 'User address not available for fetching open positions.', userAddress: null } });
    return;
  }

  dispatch({ type: FETCH_OPEN_POSITIONS_REQUEST, payload: { userAddress: effectiveUserAddress } });
  try {
    const response = await api.getOpenPositions(); // API call no longer takes userAddress
    if (response.data.status === 0 && response.data.data) {
      dispatch({
        type: FETCH_OPEN_POSITIONS_SUCCESS,
        // Store by effectiveUserAddress for consistency, even if API infers it
        payload: { positions: response.data.data, userAddress: effectiveUserAddress }
      });
      return response.data.data;
    } else {
      const errorMsg = response.data.desc || 'Failed to fetch open positions';
      dispatch({ type: FETCH_OPEN_POSITIONS_FAILURE, payload: { error: errorMsg, userAddress: effectiveUserAddress } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: FETCH_OPEN_POSITIONS_FAILURE, payload: { error: error.message, userAddress: effectiveUserAddress } });
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

export const withdrawCollateral = (marketID, userAddress, assetAddress, assetSymbol, amount) => async (dispatch, getState) => {
  dispatch({ type: WITHDRAW_COLLATERAL_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.withdrawFromCollateral({ marketID, assetAddress, amount: amount.toString() });
    if (response.data.status === 0 && response.data.data) {
      const unsignedTxData = response.data.data;
      dispatch({ type: WITHDRAW_COLLATERAL_UNSIGNED_TX_RECEIVED, payload: unsignedTxData });
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_PENDING });
      try {
        const signedTxHex = await walletService.signTransaction(unsignedTxData); // getState removed
        if (signedTxHex) {
          const market = getState().market.markets[marketID]; // getState still needed here
          const baseAssetSymbol = market ? market.baseTokenSymbol : null;
          const quoteAssetSymbol = market ? market.quoteTokenSymbol : null;
          dispatch(broadcastMarginTransaction(signedTxHex, marketID, userAddress, assetSymbol, baseAssetSymbol, quoteAssetSymbol));
        } else {
          dispatch({ type: WITHDRAW_COLLATERAL_FAILURE, payload: { error: 'Transaction signing failed or rejected by user.' } });
          dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
        }
      } catch (signError) {
        console.error('Signing error during withdraw collateral:', signError);
        dispatch({ type: WITHDRAW_COLLATERAL_FAILURE, payload: { error: `Transaction signing failed: ${signError.message || signError}` } });
        dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      }
      return unsignedTxData;
    } else {
      const errorMsg = response.data.desc || 'Failed to initiate withdraw collateral';
      dispatch({ type: WITHDRAW_COLLATERAL_FAILURE, payload: { error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: WITHDRAW_COLLATERAL_FAILURE, payload: { error: error.message } });
    throw error;
  }
};

export const borrowLoanAction = (marketID, userAddress, assetAddress, assetSymbol, amount) => async (dispatch, getState) => {
  dispatch({ type: BORROW_LOAN_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.borrowLoan({ marketID, assetAddress, amount: amount.toString() });
    if (response.data.status === 0 && response.data.data) {
      const unsignedTxData = response.data.data;
      dispatch({ type: BORROW_LOAN_UNSIGNED_TX_RECEIVED, payload: unsignedTxData });
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_PENDING });
      try {
        const signedTxHex = await walletService.signTransaction(unsignedTxData); // getState removed
        if (signedTxHex) {
          const market = getState().market.markets[marketID]; // getState still needed here
          const baseAssetSymbol = market ? market.baseTokenSymbol : null;
          const quoteAssetSymbol = market ? market.quoteTokenSymbol : null;
          dispatch(broadcastMarginTransaction(signedTxHex, marketID, userAddress, assetSymbol, baseAssetSymbol, quoteAssetSymbol));
        } else {
          dispatch({ type: BORROW_LOAN_FAILURE, payload: { error: 'Transaction signing failed or rejected by user.' } });
          dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
        }
      } catch (signError) {
        console.error('Signing error during borrow loan:', signError);
        dispatch({ type: BORROW_LOAN_FAILURE, payload: { error: `Transaction signing failed: ${signError.message || signError}` } });
        dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      }
      return unsignedTxData;
    } else {
      const errorMsg = response.data.desc || 'Failed to initiate borrow loan';
      dispatch({ type: BORROW_LOAN_FAILURE, payload: { error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: BORROW_LOAN_FAILURE, payload: { error: error.message } });
    throw error;
  }
};

export const repayLoanAction = (marketID, userAddress, assetAddress, assetSymbol, amount) => async (dispatch, getState) => {
  dispatch({ type: REPAY_LOAN_REQUEST, payload: { marketID, assetAddress, amount } });
  try {
    const response = await api.repayLoan({ marketID, assetAddress, amount: amount.toString() });
    if (response.data.status === 0 && response.data.data) {
      const unsignedTxData = response.data.data;
      dispatch({ type: REPAY_LOAN_UNSIGNED_TX_RECEIVED, payload: unsignedTxData });
      dispatch({ type: SIGNING_MARGIN_TRANSACTION_PENDING });
      try {
        const signedTxHex = await walletService.signTransaction(unsignedTxData); // getState removed
        if (signedTxHex) {
          const market = getState().market.markets[marketID]; // getState still needed here
          const baseAssetSymbol = market ? market.baseTokenSymbol : null;
          const quoteAssetSymbol = market ? market.quoteTokenSymbol : null;
          dispatch(broadcastMarginTransaction(signedTxHex, marketID, userAddress, assetSymbol, baseAssetSymbol, quoteAssetSymbol));
        } else {
          dispatch({ type: REPAY_LOAN_FAILURE, payload: { error: 'Transaction signing failed or rejected by user.' } });
          dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
        }
      } catch (signError) {
        console.error('Signing error during repay loan:', signError);
        dispatch({ type: REPAY_LOAN_FAILURE, payload: { error: `Transaction signing failed: ${signError.message || signError}` } });
        dispatch({ type: SIGNING_MARGIN_TRANSACTION_COMPLETE });
      }
      return unsignedTxData;
    } else {
      const errorMsg = response.data.desc || 'Failed to initiate repay loan';
      dispatch({ type: REPAY_LOAN_FAILURE, payload: { error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: REPAY_LOAN_FAILURE, payload: { error: error.message } });
    throw error;
  }
};

export const fetchLoans = (marketID, userAddress) => async (dispatch, getState) => { // userAddress for redux state, marketID for API
  // If userAddress is not passed, try to get it from the selected account in wallet state
  const effectiveUserAddress = userAddress || (getSelectedAccountWallet(getState())?.address);
  if (!effectiveUserAddress) {
    dispatch({ type: FETCH_LOANS_FAILURE, payload: { marketID, userAddress: null, error: 'User address not available for fetching loans.' } });
    return;
  }

  dispatch({ type: FETCH_LOANS_REQUEST, payload: { marketID, userAddress: effectiveUserAddress } });
  try {
    const response = await api.getLoans(marketID); // API call takes optional marketID
    // Assuming backend common response structure: { status: 0, data: [...] }
    if (response.data.status === 0 && response.data.data) {
      dispatch({
        type: FETCH_LOANS_SUCCESS,
        payload: { marketID, userAddress: effectiveUserAddress, loans: response.data.data }
      });
      return response.data.data;
    } else {
      const errorMsg = response.data.desc || 'Failed to fetch loans';
      dispatch({ type: FETCH_LOANS_FAILURE, payload: { marketID, userAddress: effectiveUserAddress, error: errorMsg } });
      throw new Error(errorMsg);
    }
  } catch (error) {
    dispatch({ type: FETCH_LOANS_FAILURE, payload: { marketID, userAddress: effectiveUserAddress, error: error.message } });
    throw error;
  }
};
// Make sure to keep other existing actions if this overwrite is partial.
// For this operation, I'm providing the assumed full content with modifications.
