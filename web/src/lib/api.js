import axios from 'axios';
import env from './env';
import { store } from '../index';
import { cleanLoginDate, loadAccountHydroAuthentication } from './session';
import { logout } from '../actions/account';
import { getSelectedAccount } from '@gongddex/hydro-sdk-wallet';

const getAxiosInstance = () => {
  const state = store.getState();
  const selectedAccount = getSelectedAccount(state);
  const address = selectedAccount ? selectedAccount.get('address') : null;
  const hydroAuthentication = loadAccountHydroAuthentication(address);
  let instance;

  if (hydroAuthentication) {
    instance = axios.create({
      headers: {
        'Hydro-Authentication': hydroAuthentication
      }
    });
  } else {
    instance = axios;
  }

  instance.interceptors.response.use(function(response) {
    if (response.data && response.data.status === -11) {
      if (address) {
        store.dispatch(logout(address));
        cleanLoginDate(address);
      }
    }
    return response;
  });

  return instance;
};

const _request = (method, url, ...args) => {
  return getAxiosInstance()[method](`${env.API_ADDRESS}${url}`, ...args);
};

const api = {
  get: (url, ...args) => _request('get', url, ...args),
  delete: (url, ...args) => _request('delete', url, ...args),
  head: (url, ...args) => _request('head', url, ...args),
  post: (url, ...args) => _request('post', url, ...args),
  put: (url, ...args) => _request('put', url, ...args),
  patch: (url, ...args) => _request('patch', url, ...args),

  // Margin Account Details
  getMarginAccountDetails: (marketID, userAddress) => {
    return _request('get', `/margin/accounts/${marketID}?user=${userAddress}`);
  },

  // Collateral Management
  depositToCollateral: (data) => { // data: { marketID, assetAddress, amount }
    return _request('post', '/margin/collateral/deposit', data);
  },
  withdrawFromCollateral: (data) => { // data: { marketID, assetAddress, amount }
    return _request('post', '/margin/collateral/withdraw', data);
  },

  // Loan Management
  borrowLoan: (data) => { // data: { marketID, assetAddress, amount }
    return _request('post', '/margin/loans/borrow', data);
  },
  repayLoan: (data) => { // data: { marketID, assetAddress, amount }
    return _request('post', '/margin/loans/repay', data);
  },
  getLoans: (marketID = null) => { // userAddress removed, handled by auth; marketID is optional
    let url = '/v1/margin/loans?includeInterestRates=true';
    if (marketID) {
      url += `&marketID=${marketID}`;
    }
    return _request('get', url);
  },

  // Open Margin Position
  openMarginPosition: (params) => { // params: { marketID, side, amount, price, leverage, collateralAssetSymbol, collateralAmount, userAddress }
    return _request('post', '/v1/margin/positions/open', params);
  },

  // Broadcast Transaction
  broadcastTransaction: (data) => { // data: { signedRawTx } or { signedRawTxHex } depending on backend
    return _request('post', '/v1/transactions/broadcast', data);
  },

  // Spendable Margin Balance
  getSpendableMarginBalance: (marketID, assetSymbol, userAddress) => {
    return _request('get', `/v1/margin/accounts/${marketID}/transferable-balance?assetSymbol=${assetSymbol}&userAddress=${userAddress}`);
  },

  // Open Positions (Margin)
  getOpenPositions: () => { // userAddress removed, handled by auth
    return _request('get', '/v1/margin/positions');
  },

  // Market Margin Parameters
  getMarketMarginParameters: (marketID) => {
    return _request('get', `/v1/markets/${marketID}/margin-parameters`);
  },

  // Close Margin Position
  closeMarginPosition: (params) => { // params: { marketID } (userAddress implied by auth)
    return _request('post', '/v1/margin/positions/close', params);
  },

  // Initiate Repay Loan Action (to get unsigned tx)
  repayLoanAction: (params) => { // params: { marketID, assetAddress, amount }
    return _request('post', '/v1/margin/loans/repay-action', params);
  }
};

export default api;
