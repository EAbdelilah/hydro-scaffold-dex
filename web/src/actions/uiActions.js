// Action Type Constants
export const PREFILL_TRADE_FORM_FOR_CLOSE = 'ui/PREFILL_TRADE_FORM_FOR_CLOSE';
export const CLEAR_TRADE_FORM_PREFILL = 'ui/CLEAR_TRADE_FORM_PREFILL';

// Action Creators
export const prefillTradeFormForClose = (positionDetails) => ({
  type: PREFILL_TRADE_FORM_FOR_CLOSE,
  payload: positionDetails
});

export const clearTradeFormPrefill = () => ({
  type: CLEAR_TRADE_FORM_PREFILL
});
