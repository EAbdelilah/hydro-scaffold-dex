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
      // Conceptual Backend API Endpoint: GET /v1/markets/:marketID/margin-parameters
      // OR extend existing GET /v1/markets/:marketID
      // For now, assuming a dedicated endpoint:
      // const response = await api.get(`/markets/${marketID}/margin-parameters`);
      // Mocked response for now:
      const response = { 
        data: { 
          status: 0, 
          data: {
            marketID,
            initialMarginFraction: "0.2", // Example
            maintenanceMarginFraction: "0.1", // Example
            liquidateRate: "1.1", // Example
            borrowEnableBase: true,
            borrowEnableQuote: true,
            baseAssetBorrowAPY: "0.05", // Example
            quoteAssetBorrowAPY: "0.03" // Example
          }
        }
      }; 

      if (response.data.status === 0) {
        // Format numerical strings to BigNumber if needed by components, or leave as strings
        const parameters = {
          ...response.data.data,
          // Example: Convert to BigNumber if components expect it
          // initialMarginFraction: new BigNumber(response.data.data.initialMarginFraction),
          // maintenanceMarginFraction: new BigNumber(response.data.data.maintenanceMarginFraction),
          // liquidateRate: new BigNumber(response.data.data.liquidateRate),
          // baseAssetBorrowAPY: new BigNumber(response.data.data.baseAssetBorrowAPY),
          // quoteAssetBorrowAPY: new BigNumber(response.data.data.quoteAssetBorrowAPY),
        };
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
