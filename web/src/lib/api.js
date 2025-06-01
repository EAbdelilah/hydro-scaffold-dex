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
  getLoans: (marketID, userAddress) => {
    return _request('get', `/margin/loans?marketID=${marketID}&user=${userAddress}`);
  }
};

export default api;
