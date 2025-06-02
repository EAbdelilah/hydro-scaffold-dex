import BigNumber from 'bignumber.js';
import api from '../lib/api';

export const updateCurrentMarket = currentMarket => {
  return async dispatch => {
    return dispatch({
      type: 'UPDATE_CURRENT_MARKET',
      payload: { currentMarket }
    });
  };
};

export const loadMarkets = () => {
  return async (dispatch, getState) => {
    const res = await api.get(`/markets`);
    if (res.data.status === 0) {
      const markets = res.data.data.markets;
      markets.forEach(formatMarket);
      return dispatch({
        type: 'LOAD_MARKETS',
        payload: { markets }
      });
    }
  };
};

// load current market trade history
export const loadTradeHistory = marketID => {
  return async (dispatch, getState) => {
    const res = await api.get(`/markets/${marketID}/trades`);
    const currentMarket = getState().market.getIn(['markets', 'currentMarket']);
    if (currentMarket.id === marketID) {
      return dispatch({
        type: 'LOAD_TRADE_HISTORY',
        payload: res.data.data.trades
      });
    }
  };
};

export const FETCH_MARKET_MARGIN_PARAMETERS_REQUEST = 'market/FETCH_MARKET_MARGIN_PARAMETERS_REQUEST';
export const FETCH_MARKET_MARGIN_PARAMETERS_SUCCESS = 'market/FETCH_MARKET_MARGIN_PARAMETERS_SUCCESS';
export const FETCH_MARKET_MARGIN_PARAMETERS_FAILURE = 'market/FETCH_MARKET_MARGIN_PARAMETERS_FAILURE';

export const fetchMarketMarginParameters = marketID => {
  return async dispatch => {
    dispatch({ type: FETCH_MARKET_MARGIN_PARAMETERS_REQUEST, payload: { marketID } });
    try {
      // Uses api.getMarketMarginParameters from web/src/lib/api.js
      const response = await api.getMarketMarginParameters(marketID); 

      if (response.data.status === 0 && response.data.data) { 
        const parameters = response.data.data; // Direct use of data object
        dispatch({
          type: FETCH_MARKET_MARGIN_PARAMETERS_SUCCESS,
          payload: { marketID, parameters } 
        });
        return parameters;
      } else {
        const errorMsg = response.data.desc || 'Failed to fetch market margin parameters';
        dispatch({ type: FETCH_MARKET_MARGIN_PARAMETERS_FAILURE, payload: { marketID, error: errorMsg } });
        throw new Error(errorMsg);
      }
    } catch (error) {
      dispatch({ type: FETCH_MARKET_MARGIN_PARAMETERS_FAILURE, payload: { marketID, error: error.message } });
      throw error;
    }
  };
};

const formatMarket = market => {
  market.gasFeeAmount = new BigNumber(market.gasFeeAmount);
  market.asMakerFeeRate = new BigNumber(market.asMakerFeeRate);
  market.asTakerFeeRate = new BigNumber(market.asTakerFeeRate);
  market.marketOrderMaxSlippage = new BigNumber(market.marketOrderMaxSlippage);
};
