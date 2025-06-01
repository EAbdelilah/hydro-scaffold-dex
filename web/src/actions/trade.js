import api from '../lib/api';
import { getSelectedAccountWallet } from '@gongddex/hydro-sdk-wallet';
import { getSelectedAccountType } from '../selectors/marginSelectors'; // Add this

export const TRADE_FORM_ID = 'TRADE';
// Action to clear form, if not already existing elsewhere
export const clearTradeForm = () => ({ type: '@@redux-form/CLEAR_FIELDS', meta: { form: TRADE_FORM_ID, keepTouched: false, persistentSubmitErrors: false, fields: [] }});


export const trade = (side, price, amount, orderType = 'limit', expires = 86400 * 365 * 1000) => {
  return async (dispatch, getState) => {
    try {
      const result = await dispatch(createOrder(side, price, amount, orderType, expires));
      if (result.status === 0) {
        alert('Successfully created order');
        return true;
      } else {
        alert(result.desc);
      }
    } catch (e) {
      alert(e);
    }

    return false;
  };
};

const createOrder = (side, price, amount, orderType, expires) => {
  return async (dispatch, getState) => {
    const state = getState();
    const currentMarket = state.market.getIn(['markets', 'currentMarket']);
    const selectedAccountType = getSelectedAccountType(state); // Get selected account type

    const buildOrderPayload = {
      amount: amount.toString(), // Ensure amounts/prices are strings if API expects that
      price: price.toString(),
      side,
      expires,
      orderType,
      marketID: currentMarket.id
    };

    if (selectedAccountType === 'margin' && currentMarket && currentMarket.get('borrowEnable')) {
      buildOrderPayload.accountType = 'margin';
      // marginMarketID is typically the same as marketID for the current context of placing an order in that market's margin pool
      buildOrderPayload.marginMarketID = currentMarket.id;
    }
    // If not 'margin', accountType can be omitted as backend `BuildOrderReq` has it as omitempty.

    const buildOrderResponse = await api.post('/orders/build', buildOrderPayload); // Use the new payload

    if (buildOrderResponse.data.status !== 0) {
      return buildOrderResponse.data;
    }

    const orderParams = buildOrderResponse.data.data.order;
    const { id: orderID } = orderParams;
    try {
      const wallet = getSelectedAccountWallet(state);
      const signature = await wallet.signPersonalMessage(orderID);
      const orderSignature = '0x' + signature.slice(130) + '0'.repeat(62) + signature.slice(2, 130);
      const placeOrderResponse = await api.post('/orders', {
        orderID,
        signature: orderSignature,
        method: 0
      });

      return placeOrderResponse.data;
    } catch (e) {
      alert(e);
    }
  };
};

export const tradeUpdate = trade => {
  return {
    type: 'TRADE_UPDATE',
    payload: {
      trade
    }
  };
};

export const marketTrade = trade => {
  return {
    type: 'MARKET_TRADE',
    payload: {
      trade
    }
  };
};
