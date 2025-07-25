import { Map, List, OrderedMap, fromJS } from 'immutable';

const initialOrderbook = Map({
  bids: List(),
  asks: List()
});

const initialState = Map({
  marketStatus: Map({
    loaded: false,
    loading: true,
    data: List()
  }),

  markets: Map({
    loaded: false,
    loading: true,
    data: List(),
    currentMarket: null,
    onlyMarket: null,
    baseToken: 'ALL',
    searchTerm: ''
  }),

  orderbook: initialOrderbook,
  tickers: Map({
    loading: false,
    data: {}
  }),

  isAllTradesLoading: true,
  tradeHistory: List(),

  tokenPrices: Map({
    loading: true,
    data: {}
  })
});

const reverseBigNumberComparator = (a, b) => {
  if (a[0].gt(b[0])) {
    return -1;
  } else if (a[0].eq(b[0])) {
    return 0;
  } else {
    return 1;
  }
};

export default (state = initialState, action) => {
  switch (action.type) {
    case 'LOAD_MARKETS':
      state = state.setIn(['markets', 'data'], List(action.payload.markets));
      if (!state.getIn(['markets', 'currentMarket'])) {
        state = state.setIn(['markets', 'currentMarket'], action.payload.markets[0]);
      }
      return state;
    case 'UPDATE_CURRENT_MARKET': {
      const currentMarket = action.payload.currentMarket;
      const { asTakerFeeRate, asMakerFeeRate, gasFeeAmount } = currentMarket;
      state = state.setIn(['markets', 'currentMarket'], currentMarket);
      state = state.setIn(['markets', 'currentMarketFees'], { asTakerFeeRate, asMakerFeeRate, gasFeeAmount });
      state = state.set('orderbook', initialOrderbook);
      state = state.set('tradeHistory', OrderedMap());
      return state;
    }
    case 'LOAD_TRADE_HISTORY':
      state = state.set('tradeHistory', OrderedMap());
      action.payload.reverse().forEach(t => {
        state = state.setIn(['tradeHistory', t.id], t);
      });
      return state;
    case 'MARKET_TRADE': {
      let trade = action.payload.trade;
      state = state.setIn(['tradeHistory', trade.id], trade);
      return state;
    }
    case 'INIT_ORDERBOOK':
      state = state.setIn(['orderbook', 'bids'], List(action.payload.bids).sort(reverseBigNumberComparator));
      state = state.setIn(['orderbook', 'asks'], List(action.payload.asks).sort(reverseBigNumberComparator));
      return state;
    case 'UPDATE_ORDERBOOK':
      const side = action.payload.side === 'buy' ? 'bids' : 'asks';
      const { price, amount } = action.payload;
      const index = state.getIn(['orderbook', side]).findIndex(priceLevel => priceLevel[0].eq(price));

      if (index >= 0) {
        if (amount.lte('0')) {
          state = state.deleteIn(['orderbook', side, index]);
        } else {
          state = state.updateIn(['orderbook', side, index], priceLevel => [priceLevel[0], amount]);
        }
      } else if (amount.gt('0')) {
        state = state.updateIn(['orderbook', side], list => list.push([price, amount]));
      }

      state = state.setIn(['orderbook', side], state.getIn(['orderbook', side]).sort(reverseBigNumberComparator));
      return state;
    case 'market/FETCH_MARKET_MARGIN_PARAMETERS_SUCCESS': { // Matches action type from marketActions.js
      const { marketID, parameters } = action.payload;
      // Ensure the market exists in the state before trying to set parameters
      const marketIndex = state.getIn(['markets', 'data']).findIndex(m => m.id === marketID);
      if (marketIndex !== -1) {
        state = state.setIn(['markets', 'data', marketIndex, 'marginParams'], fromJS(parameters));
        // If this is the current market, also update it
        const currentMarket = state.getIn(['markets', 'currentMarket']);
        if (currentMarket && currentMarket.id === marketID) {
          state = state.setIn(['markets', 'currentMarket', 'marginParams'], fromJS(parameters));
        }
      }
      return state;
    }
    // TODO: Handle FETCH_MARKET_MARGIN_PARAMETERS_REQUEST and FETCH_MARKET_MARGIN_PARAMETERS_FAILURE
    // to set loading/error states if needed, e.g.,
    // case 'market/FETCH_MARKET_MARGIN_PARAMETERS_REQUEST':
    //   return state.setIn(['markets', 'marginParamsLoading', action.payload.marketID], true);
    // case 'market/FETCH_MARKET_MARGIN_PARAMETERS_FAILURE':
    //   return state.setIn(['markets', 'marginParamsLoading', action.payload.marketID], false)
    //               .setIn(['markets', 'marginParamsError', action.payload.marketID], action.payload.error);
    default:
      return state;
  }
};
