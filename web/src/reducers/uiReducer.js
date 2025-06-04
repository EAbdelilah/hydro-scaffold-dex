import { fromJS } from 'immutable';
import { LOCATION_CHANGE } from 'connected-react-router'; // Assuming connected-react-router is used
import {
  PREFILL_TRADE_FORM_FOR_CLOSE,
  CLEAR_TRADE_FORM_PREFILL
} from '../actions/uiActions';

const initialState = fromJS({
  tradeFormPrefill: {
    active: false,
    marketID: null,
    side: null, // This will be the side for the closing trade (opposite of position)
    amount: null,
    price: null,
    baseAssetSymbol: null,
    quoteAssetSymbol: null,
    isClosingTradeContext: false // Added new field
  }
});

export default function uiReducer(state = initialState, action) {
  switch (action.type) {
    case PREFILL_TRADE_FORM_FOR_CLOSE: {
      const { marketID, originalSide, sizeToClose, baseAssetSymbol, quoteAssetSymbol, closePriceSuggestion } = action.payload;
      const closingSide = originalSide && originalSide.toLowerCase() === 'long' ? 'sell' : 'buy';
      return state.set('tradeFormPrefill', fromJS({
        active: true,
        marketID,
        side: closingSide,
        amount: sizeToClose ? sizeToClose.toString() : '', // Ensure it's a string
        price: closePriceSuggestion ? closePriceSuggestion.toString() : '', // Ensure it's a string
        baseAssetSymbol,
        quoteAssetSymbol,
        isClosingTradeContext: true // Set context to true
      }));
    }
    case CLEAR_TRADE_FORM_PREFILL:
      // Ensure full reset including the new field
      return state.set('tradeFormPrefill', initialState.get('tradeFormPrefill').merge(fromJS({ isClosingTradeContext: false, active: false })));

    case LOCATION_CHANGE: // Clear prefill on navigation
      // Ensure full reset including the new field
      return state.set('tradeFormPrefill', initialState.get('tradeFormPrefill').merge(fromJS({ isClosingTradeContext: false, active: false })));

    default:
      return state;
  }
}
