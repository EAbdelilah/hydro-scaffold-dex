import React, { Component } from 'react';
import { connect } from 'react-redux';
import { BigNumber } from 'bignumber.js';
import {
  openMarginPosition,
} from '../../actions/marginActions';
import { createOrder } from '../../actions/trade'; // Import createOrder
import { getSelectedAccountType } from '../../selectors/marginSelectors';
import { clearTradeFormPrefill } from '../../actions/uiActions'; // Import new action
import { selectAccountType } from '../../actions/marginActions'; // For switching to margin
import { setCurrentMarket } from '../../actions/market'; // Assuming this action exists

// Conceptual: For a real form, you'd likely use redux-form or a similar library
// and connect form values directly rather than local component state for inputs.
class Trade extends Component {
  constructor(props) {
    super(props);
    this.state = {
      side: 'buy',
      amount: '', // Default to empty for prefill
      price: '',  // Default to empty for prefill
      leverage: '2.5',
      orderType: 'limit', // Default order type
    };
  }

  componentDidUpdate(prevProps) {
    const { dispatch, tradeFormPrefill, currentMarket } = this.props; // Added dispatch here
    if (tradeFormPrefill && tradeFormPrefill.get('active') &&
        (!prevProps.tradeFormPrefill || !prevProps.tradeFormPrefill.get('active'))) {

        const prefillData = tradeFormPrefill;

        // Ensure current market is set to prefillData.get('marketID')
        if (!currentMarket || currentMarket.id !== prefillData.get('marketID')) {
           dispatch(setCurrentMarket(prefillData.get('marketID')));
        }

        dispatch(selectAccountType('margin')); // Switch to margin mode
        this.setState({
          side: prefillData.get('side'),
          amount: prefillData.get('amount'),
          price: prefillData.get('price') || '', // Use suggested price or empty for limit
          // leverage: this.state.leverage, // Keep current leverage or reset? For closing, leverage isn't directly used in form.
        });
        // Potentially set orderType to 'limit' or 'market' based on prefill or default.
        // For closing, it's usually a limit or market order to take liquidity.
        // this.setState({ orderType: 'limit' });

        dispatch(clearTradeFormPrefill()); // Clear the prefill state
    }

    // VERIFY_WS_UPDATE:
    // if (this.props.marginAccountDetails !== prevProps.marginAccountDetails) { // Assuming marginAccountDetails is mapped from getMarginAccountDetailsData
    //   console.log('Trade.js: marginAccountDetails updated via WebSocket, re-running calculations.');
    //   // this.updateMarginTradeCalculations(); // Example if calculations for collateral are done client-side
    // }
    // Also, if order book data (used for price clicking) is updated via WS, ensure it reflects if integrated.
  }

  componentWillUnmount() {
    this.props.dispatch(clearTradeFormPrefill());
  }

  // componentDidUpdate(prevProps) { // Existing one is fine, just add comment conceptually
  //   ...
  //   // VERIFY_WS_UPDATE: If prevProps.assetsTotalUSD !== this.props.assetsTotalUSD (due to WebSocket MARGIN_ACCOUNT_UPDATE),
  //   // ensure updateMarginTradeCalculations() (if such a helper exists for re-calculating collateral requirements etc.)
  //   // is called and UI reflects changes if it affects form validation or displayed available balances.
  //   // Also, if order book data (used for price clicking) is updated via WS, ensure it reflects.
  // }

  handleInputChange = (event) => {
    this.setState({ [event.target.name]: event.target.value });
  }

  handleSubmit = async (event) => {
    event.preventDefault();
    const {
      dispatch,
      selectedAccountType,
      currentMarket,
      userAddress,
      // For spendable balance check (conceptual)
      // baseAssetSymbol,
      // quoteAssetSymbol,
      // getSpendableCommonBalance // Selector needed if checking common balance
    } = this.props;

    const { side, amount, price, leverage } = this.state;

    if (!currentMarket || !userAddress) {
      alert('Market or User Address not available. Please connect your wallet and select a market.');
      return;
    }

    // Convert inputs to BigNumber for validation and calculation
    const amountBN = new BigNumber(amount);
    const priceBN = new BigNumber(price);
    // Leverage is not directly used when placing a standard order to close a position.
    // const leverageBN = selectedAccountType === 'margin' ? new BigNumber(leverage) : new BigNumber(0);

    if (amountBN.lte(0) || (priceBN.lte(0) && this.state.orderType !== 'market')) {
      alert('Amount and Price must be greater than 0 for limit orders.');
      return;
    }

    // Default expires for orders (e.g., 100 years, as in existing trade action)
    const expires = 86400 * 365 * 1000;

    if (selectedAccountType === 'margin') {
      console.log('Submitting standard order on margin account.');
      // The createOrder action (from actions/trade.js) already handles:
      // - setting accountType: 'margin'
      // - setting marginMarketID: currentMarket.id
      // These are sent to /orders/build, which should use GenerateMarginOrderDataHex.
      try {
        const result = await dispatch(createOrder(side, priceBN, amountBN, this.state.orderType, expires));
        if (result && result.status === 0) {
          console.log('Margin account order placed successfully via createOrder:', result);
          // TODO: Clear local form state if createOrder was successful.
          // Example: this.setState({ amount: '', price: '' });

          // Check if this was a closing trade to dispatch specific notification
          // if (this.props.isClosingTradeContext) { // isClosingTradeContext is true when form was prefilled for close
          //   this.props.dispatch(showMarginAlert({
          //     level: 'info', // Or 'success'
          //     message: 'Closing trade submitted. Once confirmed and position is flat, you can "Settle & Withdraw".',
          //     autoDismiss: 10000
          //   }));
          // }
          // Note: The actual dispatch of this notification is better handled within the createOrder/trade success logic
          // if it can be aware of the 'isClosingTradeContext'. For now, this comment serves as a placeholder for the idea.
        } else {
          console.error('Margin account order placement failed via createOrder:', result ? result.desc : 'Unknown error');
          // Alerts are handled by createOrder
        }
      } catch (error) {
        console.error('Dispatching createOrder for margin account failed:', error);
        alert(`Error placing order on margin account: ${error.message || 'Unknown error'}`);
      }
    } else { // Spot Trading
      console.log('Executing spot trade logic using createOrder.');
      try {
        const result = await dispatch(createOrder(side, priceBN, amountBN, this.state.orderType, expires));
        if (result && result.status === 0) {
          console.log('Spot order placed successfully via createOrder:', result);
          // TODO: Clear local form state if createOrder was successful for spot trades.
          // Example: this.setState({ amount: '', price: '' });
        } else {
          console.error('Spot order placement failed via createOrder:', result ? result.desc : 'Unknown error');
        }
      } catch (error) {
        console.error('Dispatching createOrder for spot account failed:', error);
        alert(`Error placing spot order: ${error.message || 'Unknown error'}`);
      }
    }
  };

  render() {
    const {
      selectedAccountType,
      isOpeningMarginPosition, // This state is for the openMarginPosition action
      isClosingMarginPosition,
      isSigningInWallet,      // This state is for marginActions (open/close/deposit etc.)
      isBroadcastingMarginTx, // This state is for marginActions
      marginActionError,
      unsignedMarginTx,
      currentMarket,
      isClosingTradeContext, // Mapped from state.ui.tradeFormPrefill
      // TODO: Add props for standard order loading states if createOrder has them
      // e.g., isBuildingOrder, isPlacingOrder, orderError from a different part of Redux state
    } = this.props;

    // isProcessingMarginAction is specific to complex margin operations, not standard orders.
    const isProcessingComplexMarginAction = isOpeningMarginPosition || isClosingMarginPosition || isSigningInWallet || isBroadcastingMarginTx;

    const isFormInputValid = parseFloat(this.state.amount) > 0 &&
                             (this.state.orderType === 'market' || parseFloat(this.state.price) > 0);

    const submitButtonText = isClosingTradeContext
      ? 'Submit Closing Trade'
      : selectedAccountType === 'margin'
        ? 'Place Margin Order'
        : 'Place Order';

    // TODO: The loading/feedback UI below is for the `openMarginPosition` flow.
    // If `createOrder` flow is used, it currently relies on alerts.
    // For a better UX, `createOrder` should dispatch actions to set loading/error/success states
    // in Redux, and this component should map and display them.
    // For this subtask, we'll keep the existing margin feedback UI, acknowledging it won't
    // reflect the state of `createOrder` calls.

    return (
      <div style={{ border: '1px solid #ccc', padding: '20px', margin: '20px' }}>
        <h2>Trade ({selectedAccountType})</h2>
        <form onSubmit={this.handleSubmit}>
          <div>Market: {currentMarket ? currentMarket.id : 'N/A'}</div>
          <div>
            <label>Side:</label>
            <select name="side" value={this.state.side} onChange={this.handleInputChange}>
              <option value="buy">Buy</option>
              <option value="sell">Sell</option>
            </select>
          </div>
          <div>
            <label>Price ({currentMarket ? currentMarket.get('quoteToken','Quote') : ''}):</label>
            <input type="text" name="price" value={this.state.price} onChange={this.handleInputChange} />
          </div>
          <div>
            <label>Amount ({currentMarket ? currentMarket.get('baseToken','Base') : ''}):</label>
            <input type="text" name="amount" value={this.state.amount} onChange={this.handleInputChange} />
          </div>
          {selectedAccountType === 'margin' && (
            <div>
              {/* Leverage is not directly used for placing a simple closing order on margin account.
                  It's for opening new leveraged positions with the openMarginPosition action.
                  Keeping it visible for now, but it won't be used by createOrder. */}
              <label>Leverage (Not used for simple margin orders):</label>
              <input type="text" name="leverage" value={this.state.leverage} onChange={this.handleInputChange} />
            </div>
          )}
          <button type="submit" disabled={isProcessingComplexMarginAction || !isFormInputValid}>
            {/* Text change based on context */}
            {isProcessingComplexMarginAction ?
              (isOpeningMarginPosition ? 'Preparing Trade...' : isSigningInWallet ? 'Awaiting Signature...' : 'Processing...') :
              submitButtonText}
          </button>
        </form>

        {/* UI Feedback Section - Primarily for openMarginPosition flow */}
        {selectedAccountType === 'margin' && isProcessingComplexMarginAction && (
          <div style={{ marginTop: '15px' }}>
            {isOpeningMarginPosition && <p>Communicating with backend to prepare margin trade...</p>}
            {unsignedMarginTx && isSigningInWallet && !isBroadcastingMarginTx && (
              <div style={{ color: 'orange' }}>
                <p><strong>Action Required:</strong> Please sign the transaction in your wallet.</p>
                <details><summary>Unsigned Transaction Details (Debug)</summary>
                  <pre style={{ fontSize: '0.8em', backgroundColor: '#f0f0f0', padding: '5px' }}>
                    {JSON.stringify(unsignedMarginTx.toJS ? unsignedMarginTx.toJS() : unsignedMarginTx, null, 2)}
                  </pre>
                </details>
              </div>
            )}
            {isBroadcastingMarginTx && <p>Broadcasting transaction to the network...</p>}
          </div>
        )}
        {/* lastMarginTxHash display removed, handled by global MarginAlertDisplay now */}
        {marginActionError && selectedAccountType === 'margin' && (
          <p style={{ color: 'red' }}>
            Error (Margin Operation): {typeof marginActionError === 'object' ? JSON.stringify(marginActionError) : marginActionError.toString()}
          </p>
        )}
        {/* TODO: Add UI feedback for standard order placement (createOrder flow) */}
      </div>
    );
  }
}

const mapStateToProps = (state) => {
  const currentMarket = state.market.getIn(['markets', 'currentMarket']);
  const userAddress = state.account.get('address');
  const marginUIState = state.margin.get('ui');
  const marginRootState = state.margin;


  return {
    selectedAccountType: getSelectedAccountType(state),
    currentMarket,
    userAddress,
    // Specific UI states for opening/closing a margin position
    isOpeningMarginPosition: marginUIState.get('isOpeningMarginPosition', false),
    isClosingMarginPosition: marginUIState.get('isClosingMarginPosition', false), // For disabling button generally

    isSigningInWallet: marginUIState.get('isSigningInWallet', false),
    unsignedMarginTx: marginRootState.get('unsignedMarginTx'),
    isBroadcastingMarginTx: marginUIState.get('isBroadcastingMarginTx', false),
    marginActionError: marginUIState.get('marginActionError'), // Generalized error
    // lastMarginTxHash: marginRootState.get('lastMarginTxHash'), // Removed from here
    // Example for form submitting state if using redux-form
    // submitting: state.form[TRADE_FORM_ID] ? state.form[TRADE_FORM_ID].submitting : false,
    tradeFormPrefill: state.ui.get('tradeFormPrefill'), // Map the prefill state
    isClosingTradeContext: state.ui.getIn(['tradeFormPrefill', 'isClosingTradeContext'], false)
  };
};

export default connect(mapStateToProps)(Trade);
