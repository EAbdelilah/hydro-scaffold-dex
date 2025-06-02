import React, { Component } from 'react';
import { connect } from 'react-redux';
import { BigNumber } from 'bignumber.js';
import { 
  openMarginPosition, 
  // broadcastMarginTransaction, // Not dispatched directly from component anymore
  // initiateCloseMarginPosition // Not part of this component's direct responsibility
} from '../../actions/marginActions';
import { getSelectedAccountType } from '../../selectors/marginSelectors';
// import { TRADE_FORM_ID, clearTradeForm } from '../../actions/trade'; // clearTradeForm is dispatched by thunk

// Conceptual: For a real form, you'd likely use redux-form or a similar library
// and connect form values directly rather than local component state for inputs.
class Trade extends Component {
  constructor(props) {
    super(props);
    this.state = {
      side: 'buy', 
      amount: '1', 
      price: '100', 
      leverage: '2.5', 
    };
  }

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
    const leverageBN = selectedAccountType === 'margin' ? new BigNumber(leverage) : new BigNumber(0);

    if (amountBN.lte(0) || (priceBN.lte(0) && this.state.orderType !== 'market')) { // Assuming market order might not need price
        alert('Amount and Price must be greater than 0 for limit orders.');
        return;
    }

    if (selectedAccountType === 'margin') {
      if (leverageBN.lte(0)) {
        alert('Leverage must be greater than 0 for margin trades.');
        return;
      }
      console.log('Attempting to open margin position');
      
      const baseAssetSymbol = currentMarket.get('baseToken');
      const quoteAssetSymbol = currentMarket.get('quoteToken');
      let collateralAmountDecimal;
      let collateralAssetSymbol;

      // Simplified collateral calculation: (amount * price) / leverage
      // Assumes collateral is always in the quote currency of the market.
      // This might need adjustment based on actual margin system rules (e.g., cross-margin, specific collateral asset).
      const positionValueInQuote = amountBN.times(priceBN); 
      collateralAmountDecimal = positionValueInQuote.div(leverageBN);
      collateralAssetSymbol = quoteAssetSymbol; 
      
      // Conceptual: Client-side hint for available balance (backend will do the definitive check)
      // const spendableCollateral = getSpendableCommonBalance(this.props.state, collateralAssetSymbol); // Needs this selector
      // if (collateralAmountDecimal.gt(spendableCollateral)) {
      //   alert(`Insufficient ${collateralAssetSymbol} balance for the required collateral.`);
      //   return;
      // }

      const params = {
        marketID: currentMarket.id,
        side,
        amount: amountBN.toString(),
        price: priceBN.toString(), // Assuming limit order for now
        leverage: leverageBN.toNumber(),
        collateralAssetSymbol,
        collateralAmount: collateralAmountDecimal.toString(),
        userAddress, 
        baseAssetSymbol, // For refresh action context
        quoteAssetSymbol // For refresh action context
      };

      try {
        // Dispatch the thunk that handles API call, signing, and broadcasting
        await dispatch(openMarginPosition(params)); 
      } catch (error) {
        // Errors are handled by Redux state, but can catch here for component-specific feedback if needed
        console.error('Open margin position initiation failed (component-level catch):', error);
        // alert(`Error: ${error.message || 'Failed to initiate margin position.'}`); // Redux state should handle this
      }

    } else {
      // Existing spot trading logic would go here
      console.log('Executing spot trade logic (placeholder)');
      alert('Spot trading logic needs to be implemented.');
      // Example: dispatch(spotTradeAction({ marketID: currentMarket.id, side, amount, price }));
    }
  };

  render() {
    const { 
      selectedAccountType,
      isOpeningMarginPosition,
      isClosingMarginPosition, // For disabling button if a close is in progress
      isSigningInWallet, 
      isBroadcastingMarginTx, 
      marginActionError,
      unsignedMarginTx, 
      lastMarginTxHash,
      currentMarket
    } = this.props;

    const isProcessingMarginAction = isOpeningMarginPosition || isClosingMarginPosition || isSigningInWallet || isBroadcastingMarginTx;
    // Basic validation for enabling submit button (can be more sophisticated)
    const isFormInputValid = parseFloat(this.state.amount) > 0 && parseFloat(this.state.price) > 0 && 
                             (selectedAccountType !== 'margin' || parseFloat(this.state.leverage) > 0);


    return (
      <div style={{ border: '1px solid #ccc', padding: '20px', margin: '20px' }}>
        <h2>Trade ({selectedAccountType})</h2>
        <form onSubmit={this.handleSubmit}>
          <div>Market: {currentMarket ? currentMarket.id : 'N/A'}</div>
          {/* ... other form inputs ... */}
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
              <label>Leverage (e.g., 2, 2.5, 5):</label>
              <input type="text" name="leverage" value={this.state.leverage} onChange={this.handleInputChange} />
            </div>
          )}
          <button type="submit" disabled={isProcessingMarginAction || !isFormInputValid}>
            {isOpeningMarginPosition ? 'Preparing Trade...' : 
             isSigningInWallet ? 'Awaiting Signature...' :
             isBroadcastingMarginTx ? 'Broadcasting...' :
             (selectedAccountType === 'margin' ? 'Open Margin Position' : 'Place Spot Order')}
          </button>
        </form>

        {/* UI Feedback Section */}
        {selectedAccountType === 'margin' && (
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
            {lastMarginTxHash && !marginActionError && ( // Show only if no new error
              <p style={{ color: 'green' }}>
                Margin transaction successfully broadcasted! Hash: {lastMarginTxHash}
              </p>
            )}
            {marginActionError && (
              <p style={{ color: 'red' }}>
                Error: {typeof marginActionError === 'object' ? JSON.stringify(marginActionError) : marginActionError.toString()}
              </p>
            )}
          </div>
        )}
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
    lastMarginTxHash: marginRootState.get('lastMarginTxHash'), 
    // Example for form submitting state if using redux-form
    // submitting: state.form[TRADE_FORM_ID] ? state.form[TRADE_FORM_ID].submitting : false, 
  };
};

export default connect(mapStateToProps)(Trade);
