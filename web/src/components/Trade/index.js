import React from 'react';
import { connect } from 'react-redux';
import { formValueSelector, Field, stopSubmit, clearFields } from 'redux-form';
import { TRADE_FORM_ID, clearTradeForm } from '../../actions/trade'; // Assuming clearTradeForm exists or can be added
import { reduxForm } from 'redux-form';
import { trade } from '../../actions/trade';
import { selectAccountType } from '../../actions/marginActions';
import { getSelectedAccountType, getSpendableMarginBalance, getMarginAccountDetailsData } from '../../selectors/marginSelectors'; // Added selectors
import BigNumber from 'bignumber.js';
import { loadHotDiscountRules, getHotTokenAmount } from '../../actions/fee';
// import { getEthBalance } from '../../selectors/account'; // Not explicitly used yet, but good to note
import { calculateTrade } from '../../lib/tradeCalculator';
import { loginRequest } from '../../actions/account';
import PerfectScrollbar from 'perfect-scrollbar';
import './styles.scss';
import { sleep, toUnitAmount } from '../../lib/utils';
import { getSelectedAccount } from '@gongddex/hydro-sdk-wallet';
import { stateUtils } from '../../selectors/account';

const mapStateToProps = state => {
  const selector = formValueSelector(TRADE_FORM_ID);
  const bids = state.market.getIn(['orderbook', 'bids']);
  const asks = state.market.getIn(['orderbook', 'asks']);
  const selectedAccount = getSelectedAccount(state);
  const address = selectedAccount ? selectedAccount.get('address') : null;
  const currentMarket = state.market.getIn(['markets', 'currentMarket']);
  const lastTrade = state.market.get('tradeHistory').first();
  const lastPrice = lastTrade ? new BigNumber(lastTrade.price) : new BigNumber('0');
  const selectedAccountType = getSelectedAccountType(state);
  const isMarginEnabledMarket = currentMarket ? currentMarket.get('borrowEnable', false) : false;
  const marketID = currentMarket ? currentMarket.get('id') : null;

  let quoteTokenBalance, baseTokenBalance;
  let assetsTotalUSD = new BigNumber(0);
  let debtsTotalUSD = new BigNumber(0);

  if (marketID) {
    const marginAccountDetails = getMarginAccountDetailsData(state, marketID);
    if (marginAccountDetails && !marginAccountDetails.isEmpty()) {
      assetsTotalUSD = new BigNumber(marginAccountDetails.get('assetsTotalUSDValue', '0'));
      debtsTotalUSD = new BigNumber(marginAccountDetails.get('debtsTotalUSDValue', '0'));
    }
  }

  if (currentMarket && address) {
    const baseTokenSymbol = currentMarket.get('baseToken');
    const quoteTokenSymbol = currentMarket.get('quoteToken');
    // marketID already defined above

    if (selectedAccountType === 'margin' && isMarginEnabledMarket && marketID) {
      // For margin, get spendable margin balance (collateral + borrowed)
      // These selectors return BigNumber instances in base units.
      quoteTokenBalance = getSpendableMarginBalance(state, marketID, quoteTokenSymbol);
      baseTokenBalance = getSpendableMarginBalance(state, marketID, baseTokenSymbol);
    } else {
      // For spot, use existing logic. stateUtils.getTokenAvailableBalance returns BigNumber in base units.
      quoteTokenBalance = stateUtils.getTokenAvailableBalance(state, address, quoteTokenSymbol);
      baseTokenBalance = stateUtils.getTokenAvailableBalance(state, address, baseTokenSymbol);
    }
  } else {
    quoteTokenBalance = new BigNumber(0);
    baseTokenBalance = new BigNumber(0);
  }

  return {
    initialValues: {
      side: 'buy',
      orderType: 'limit',
      subtotal: new BigNumber(0),
      total: new BigNumber(0),
      totalBase: new BigNumber(0),
      feeRate: new BigNumber(0),
      gasFee: new BigNumber(0),
      hotDiscount: new BigNumber(1),
      tradeFee: new BigNumber(0),
      estimatedPrice: new BigNumber(0),
      marketOrderWorstPrice: new BigNumber(0),
      marketOrderWorstTotalQuote: new BigNumber(0),
      marketOrderWorstTotalBase: new BigNumber(0)
    },
    lastPrice,
    currentMarket,
    quoteTokenBalance, // Now conditional
    baseTokenBalance,  // Now conditional
    hotTokenAmount: state.config.get('hotTokenAmount'),
    address,
    isLoggedIn: state.account.getIn(['isLoggedIn', address]),
    assetsTotalUSD, // Added for margin calculations
    debtsTotalUSD,   // Added for margin calculations
    price: new BigNumber(selector(state, 'price') || 0),
    amount: new BigNumber(selector(state, 'amount') || 0),
    total: new BigNumber(selector(state, 'total') || 0),
    totalBase: new BigNumber(selector(state, 'totalBase') || 0),
    subtotal: new BigNumber(selector(state, 'subtotal') || 0),
    feeRate: new BigNumber(selector(state, 'feeRate') || 0),
    gasFee: new BigNumber(selector(state, 'gasFee') || 0),
    estimatedPrice: new BigNumber(selector(state, 'estimatedPrice') || 0),
    marketOrderWorstPrice: new BigNumber(selector(state, 'marketOrderWorstPrice') || 0),
    marketOrderWorstTotalQuote: new BigNumber(selector(state, 'marketOrderWorstTotalQuote') || 0),
    marketOrderWorstTotalBase: new BigNumber(selector(state, 'marketOrderWorstTotalBase') || 0),
    hotDiscount: new BigNumber(selector(state, 'hotDiscount') || 1),
    tradeFee: new BigNumber(selector(state, 'tradeFee') || 0),
    side: selector(state, 'side'),
    orderType: selector(state, 'orderType'),
    bestBidPrice: bids.size > 0 ? bids.get(0)[0].toString() : null,
    bestAskPrice: asks.size > 0 ? asks.get(asks.size - 1)[0].toString() : null,
    selectedAccountType, // Was already here, now sourced locally before return
    isMarginEnabledMarket
  };
};

class Trade extends React.PureComponent {
  constructor(props) {
    super(props);
    this.state = {
      leverage: 1, // Default leverage
      estimatedBorrowNeeded: new BigNumber(0),
      projectedMarginRatio: null,
      estimatedLiquidationPrice: null,
      canPlaceMarginTrade: true,
      marginCalcError: null,
    };
  }

  componentDidMount() {
    const { dispatch, currentMarket, selectedAccountType } = this.props;
    loadHotDiscountRules(); // Assuming this is a global setup
    this.interval = window.setInterval(() => {
      dispatch(getHotTokenAmount()); // Assuming this is relevant for both modes
    }, 30 * 1000);

    // Initial check in case component mounts with a non-margin market but margin mode selected
    if (currentMarket && !currentMarket.get('borrowEnable', false) && selectedAccountType === 'margin') {
      dispatch(selectAccountType('spot'));
    }
  }

  componentDidUpdate(prevProps) {
    const { currentMarket, reset, lastPrice, price, change, dispatch, selectedAccountType, isMarginEnabledMarket } = this.props;

    if (currentMarket && currentMarket.id !== prevProps.currentMarket.id) {
      // Market has changed
      reset(); // Reset the form
      // If new market doesn't support margin and current mode is margin, switch to spot
      if (!isMarginEnabledMarket && selectedAccountType === 'margin') {
        dispatch(selectAccountType('spot'));
      }
      // Set initial price if available
      if (price.eq(0) && lastPrice && !lastPrice.eq(0)) {
        change('price', lastPrice.toString());
      }
    } else if (currentMarket && currentMarket.id === prevProps.currentMarket.id) {
      // Market is the same, check if price needs update (e.g. clicking on orderbook)
      // This part of original logic: if (!lastPrice.eq(prevProps.lastPrice) && price.eq(0))
      // might need re-evaluation based on how price is set from orderbook now.
      // For now, keep it if it's meant to set initial price when market context is established.
      if (prevProps.price !== price) { // Check if price was changed by some other means (e.g. orderbook click)
        // This seems more like a sync from external price changes, not setting initial.
      }
    }

    // Check if fees need to be updated due to prop changes including selectedAccountType
    if (
      prevProps.orderType !== this.props.orderType ||
      prevProps.side !== this.props.side ||
      !prevProps.price.eq(this.props.price) ||
      !prevProps.amount.eq(this.props.amount) ||
      !prevProps.hotTokenAmount.eq(this.props.hotTokenAmount) ||
      prevProps.selectedAccountType !== this.props.selectedAccountType ||
      !prevProps.assetsTotalUSD.eq(this.props.assetsTotalUSD) || // For re-calc on account detail changes
      !prevProps.debtsTotalUSD.eq(this.props.debtsTotalUSD)
    ) {
      this.updateFees();
      this.updateMarginTradeCalculations(); // Also call this if relevant props change
    }

    // If margin becomes unsupported for the market but was selected, switch to spot
    if (prevProps.isMarginEnabledMarket && !isMarginEnabledMarket && selectedAccountType === 'margin') {
        dispatch(selectAccountType('spot'));
    }
    // If leverage changed (from this.state comparison)
    if (this.state.leverage !== prevState.leverage && selectedAccountType === 'margin') {
        this.updateMarginTradeCalculations();
    }
  }

  handleAccountTypeChange = (newType) => {
    const { dispatch, change, side } = this.props; // Assuming `change` is from redux-form to update form values
    dispatch(selectAccountType(newType));
    // Potentially clear form fields or adjust them based on account type
    // For example, margin orders might have different fee structures or leverage selectors later
    dispatch(clearFields(TRADE_FORM_ID, false, false, 'amount', 'price', 'total', 'subtotal', 'totalBase')); // Keep side
    // Re-initialize side if needed, or ensure it's preserved correctly
    change('side', side); // Explicitly set side again as clearFields might remove it
  };

  render() {
    const { side, handleSubmit, currentMarket, total, gasFee, tradeFee, subtotal, change, selectedAccountType, isMarginEnabledMarket, dispatch } = this.props;
    if (!currentMarket) {
      return null;
    }

    const currentModeName = selectedAccountType === 'margin' ? 'Margin Order' : 'Spot Order';

    return (
      <>
        <div className="title">
          <div>
            <div>{currentMarket.id}</div>
            <div className="text-secondary">Make a {currentModeName}</div>
          </div>
        </div>

        {isMarginEnabledMarket && currentMarket && ( // Ensure currentMarket is available
          <div className="market-margin-details text-secondary small mb-1 p-2" style={{borderBottom: "1px solid #222A35"}}>
            <span title="Liquidation Rate">Liq. Rate: {new BigNumber(currentMarket.get('liquidateRate', '0')).toFormat(2, BigNumber.ROUND_UP)}</span> | {' '}
            <span title="Withdraw Rate">Withdraw Rate: {new BigNumber(currentMarket.get('withdrawRate', '0')).toFormat(2, BigNumber.ROUND_UP)}</span>
            <br />
            <span title={`${currentMarket.get('baseToken', '')} Borrow APY`}>
              {currentMarket.get('baseToken', '')}-Borrow: {new BigNumber(currentMarket.get('baseTokenBorrowAPY', '0')).multipliedBy(100).toFormat(2)}%
            </span> | {' '}
            <span title={`${currentMarket.get('quoteToken', '')} Borrow APY`}>
              {currentMarket.get('quoteToken', '')}-Borrow: {new BigNumber(currentMarket.get('quoteTokenBorrowAPY', '0')).multipliedBy(100).toFormat(2)}%
            </span>
            <br />
            <span title={`${currentMarket.get('baseToken', '')} Supply APY`}>
              {currentMarket.get('baseToken', '')}-Supply: {new BigNumber(currentMarket.get('baseTokenSupplyAPY', '0')).multipliedBy(100).toFormat(2)}%
            </span> | {' '}
            <span title={`${currentMarket.get('quoteToken', '')} Supply APY`}>
              {currentMarket.get('quoteToken', '')}-Supply: {new BigNumber(currentMarket.get('quoteTokenSupplyAPY', '0')).multipliedBy(100).toFormat(2)}%
            </span>
          </div>
        )}

        <div className="trade flex-1 flex-column">
          {isMarginEnabledMarket && ( // Account Type Tabs
            <ul className="nav nav-tabs account-type-tabs">
              <li className="nav-item flex-1 flex">
                <div
                  className={`flex-1 tab-button text-center${selectedAccountType === 'spot' ? ' active' : ''}`}
                  onClick={() => this.handleAccountTypeChange('spot')}>
                  Spot
                </div>
              </li>
              <li className="nav-item flex-1 flex">
                <div
                  className={`flex-1 tab-button text-center${selectedAccountType === 'margin' ? ' active' : ''}`}
                  onClick={() => this.handleAccountTypeChange('margin')}>
                  Margin
                </div>
              </li>
            </ul>
          )}
          <ul className="nav nav-tabs"> {/* Existing Buy/Sell Tabs */}
            <li className="nav-item flex-1 flex">
              <div
                className={`flex-1 tab-button text-secondary text-center${side === 'buy' ? ' active' : ''}`}
                onClick={() => change('side', 'buy')}>
                Buy
              </div>
            </li>
            <li className="nav-item flex-1 flex">
              <div
                className={`flex-1 tab-button text-secondary text-center${side === 'sell' ? ' active' : ''}`}
                onClick={() => change('side', 'sell')}>
                Sell
              </div>
            </li>
          </ul>
          <div className="flex flex-1 position-relative overflow-hidden" ref={ref => this.setRef(ref)}>
            <form
              className="form flex-column text-secondary flex-1 justify-content-between"
              onSubmit={handleSubmit(() => this.submit())}>
              <div>
                <Field
                  name="price"
                  unit={currentMarket.quoteToken}
                  autoComplete="off"
                  component={this.renderField}
                  label="Price"
                />
                <Field
                  name="amount"
                  unit={currentMarket.baseToken}
                  autoComplete="off"
                  component={this.renderField}
                  label="Amount"
                />
                {this.props.selectedAccountType === 'margin' && this.props.isMarginEnabledMarket && (
                  <div className="form-group leverage-section">
                    <label htmlFor="leverage-input">Leverage</label>
                    <div className="input-group input-group-sm">
                      <input
                        id="leverage-input"
                        type="range"
                        className="custom-range"
                        min="1"
                        max={currentMarket.getIn(['marginParams', 'maxLeverage'], 5)} // Example: Get max leverage from market data
                        step="0.1"
                        value={this.state.leverage}
                        onChange={this.handleLeverageChange}
                      />
                       <div className="input-group-append">
                         <span className="input-group-text" style={{minWidth: "45px"}}>{this.state.leverage.toFixed(1)}x</span>
                       </div>
                    </div>
                    {selectedAccountType === 'margin' && (
                      <>
                        <p className="text-muted small mt-1 mb-0">
                          Est. Borrow: {this.state.estimatedBorrowNeeded.toFormat(currentMarket.get('quoteTokenDecimals', 2))} {currentMarket.get('quoteToken')}
                        </p>
                        {this.state.projectedMarginRatio && (
                          <p className="text-muted small mb-0">
                            Projected Ratio: {this.state.projectedMarginRatio.isFinite() ? this.state.projectedMarginRatio.toFormat(4) : 'Infinity'}
                            {this.state.projectedMarginRatio.isFinite() && this.state.projectedMarginRatio.lt(new BigNumber(currentMarket.get('liquidateRate', '0'))) ?
                            <span className="text-danger"> (Below Liq. Rate!)</span> : ''}
                          </p>
                        )}
                        {this.state.estimatedLiquidationPrice && this.state.estimatedLiquidationPrice.isPositive() && (
                           <p className="text-muted small mb-0">
                             Est. Liq. Price ({currentMarket.get('baseToken')}): {this.state.estimatedLiquidationPrice.toFormat(currentMarket.get('priceDecimals', 2))} {currentMarket.get('quoteToken')}
                           </p>
                        )}
                        {this.state.marginCalcError && <p className="text-danger small mt-1 mb-0">{this.state.marginCalcError}</p>}
                      </>
                    )}
                  </div>
                )}
                <div className="form-group">
                  <div className="form-title">Order Summary</div>
                  <div className="list">
                    <div className="item flex justify-content-between">
                      <div className="name">Order</div>
                      <div className="name">{subtotal.toFixed(currentMarket.priceDecimals)}</div>
                    </div>
                    <div className="item flex justify-content-between">
                      <div className="name">Fees</div>
                      <div className="name">{gasFee.plus(tradeFee).toFixed(currentMarket.priceDecimals)}</div>
                    </div>
                    <div className="item flex justify-content-between">
                      <div className="name">Total</div>
                      <div className="name">{total.toFixed(currentMarket.priceDecimals)}</div>
                    </div>
                  </div>
                </div>
              </div>
              <button
                type="submit"
                className={`form-control btn ${side === 'buy' ? 'btn-success' : 'btn-danger'}`}
                disabled={this.props.submitting || (this.props.selectedAccountType === 'margin' && !this.state.canPlaceMarginTrade)}
              >
                {side} {currentMarket.baseToken}
              </button>
            </form>
          </div>
        </div>
      </>
    );
  }

  renderField = ({ input, label, unit, meta, ...attrs }) => {
    const { submitFailed, error } = meta;

    return (
      <div className="form-group">
        <label>{label}</label>
        <div className="input-group">
          <input className="form-control" {...input} {...attrs} />
          <span className="text-secondary unit">{unit}</span>
        </div>
        <span className="text-danger">{submitFailed && (error && <span>{error}</span>)}</span>
      </div>
    );
  };

  async submit() {
    const { amount, price, side, orderType, dispatch, isLoggedIn, address } = this.props;
    if (!isLoggedIn) {
      await dispatch(loginRequest(address));
      // Metamask's window will be hidden when continuous call Metamask sign method
      await sleep(500);
    }
    try {
      await dispatch(trade(side, price, amount, orderType));
    } catch (e) {
      alert(e);
    }
  }

  updateFees() { // Removed prevProps, will use this.props directly
    const { currentMarket, orderType, side, price, amount, hotTokenAmount, change, selectedAccountType } = this.props;

    // TODO: Potentially adjust fee rates based on selectedAccountType if margin accounts have different fee tiers.
    // For now, using asMakerFeeRate and asTakerFeeRate from currentMarket directly.
    // This might require fetching margin-specific fee rates if they differ from spot rates.
    utils.Dump("Updating fees for account type:", selectedAccountType, "Leverage:", this.state.leverage);

    // TODO: Client-Side Calculations for Margin (Conceptual)
    // if (selectedAccountType === 'margin') {
    //   const { leverage } = this.state;
    //   const { price, amount } = this.props; // Form values (BigNumber)
    //   // const { ownCollateralBase, ownCollateralQuote } = this.getOwnCollateral(); // Conceptual
    //   const ownCollateralBase = new BigNumber(0); // Placeholder for actual unencumbered collateral selector
    //   const ownCollateralQuote = new BigNumber(0); // Placeholder
		//
    //   utils.Dump("Margin Calc: Leverage", leverage, "Price", price.toString(), "Amount", amount.toString());
    //   utils.Dump("Margin Calc: Own Collateral Base", ownCollateralBase.toString(), "Own Collateral Quote", ownCollateralQuote.toString());
		//
    //   // TODO: Calculate max position value: (ownCollateralQuote or ownCollateralBase in quote terms) * leverage
    //   // TODO: If user inputs amount, check (amount * price) against max position value.
    //   // TODO: If user inputs total position value, derive amount.
    //   // TODO: Calculate requiredBorrow = totalPositionValue - ownCollateral.
    //   // TODO: Update form state or display these calculated values (e.g., estimated borrow needed).
    //   // TODO: Calculate and display estimated liquidation price and margin ratio for the *potential* trade.
    //   //       This would involve: currentAccountDetails (total assets/debts USD), new trade value, new borrow value.
    // }


    const { asMakerFeeRate, asTakerFeeRate, gasFeeAmount, priceDecimals, amountDecimals } = currentMarket;

    const calculateParam = {
      orderType,
      side,
      price: new BigNumber(price),
      amount: new BigNumber(amount),
      hotTokenAmount,
      gasFeeAmount,
      asMakerFeeRate,
      asTakerFeeRate,
      amountDecimals,
      priceDecimals
    };

    const calculateResult = calculateTrade(calculateParam);

    change('subtotal', calculateResult.subtotal);
    change('estimatedPrice', calculateResult.estimatedPrice);
    change('totalBase', calculateResult.totalBaseTokens);
    change('total', calculateResult.totalQuoteTokens);
    change('feeRate', calculateResult.feeRateAfterDiscount);
    change('gasFee', calculateResult.gasFeeAmount);
    change('hotDiscount', calculateResult.hotDiscount);
    change('tradeFee', calculateResult.tradeFeeAfterDiscount);
  }

  handleLeverageChange = (event) => {
    const newLeverage = parseFloat(event.target.value) || 1;
    this.setState({ leverage: newLeverage }, () => {
      // TODO: Trigger recalculations of fees, max position, borrow amount, etc.
      // This might involve calling updateFees or another dedicated calculation method.
      this.updateFees(); // Re-calculate fees which might depend on leverage-affected values indirectly
      this.updateMarginTradeCalculations(); // Conceptual: for max position, borrow needs, etc.
    });
  };

  // Conceptual function for margin trade calculations
  updateMarginTradeCalculations = () => {
    const {
      price, amount, selectedAccountType, currentMarket,
      assetsTotalUSD, debtsTotalUSD, // These are for the *entire margin account* for this market
      // quoteTokenBalance, baseTokenBalance, // These are spendable balances (collateral+borrowed) for the specific asset
      side,
    } = this.props;
    const { leverage } = this.state;

    if (selectedAccountType !== 'margin' || !currentMarket || !currentMarket.get('borrowEnable') || price.isZero() || amount.isZero() || !price.isFinite() || !amount.isFinite()) {
      this.setState({
        estimatedBorrowNeeded: new BigNumber(0),
        projectedMarginRatio: null,
        estimatedLiquidationPrice: null,
        canPlaceMarginTrade: selectedAccountType === 'spot',
        marginCalcError: null,
      });
      return;
    }

    const liquidateRate = new BigNumber(currentMarket.get('liquidateRate', '0'));
    if (liquidateRate.isZero() || !liquidateRate.isFinite()) {
      this.setState({ marginCalcError: "Market liquidate rate not available or invalid.", canPlaceMarginTrade: false, estimatedBorrowNeeded: new BigNumber(0), projectedMarginRatio: null, estimatedLiquidationPrice: null });
      return;
    }

    const totalPositionValueQuote = amount.multipliedBy(price); // Value of this new position in quote currency

    if (totalPositionValueQuote.isZero() || !totalPositionValueQuote.isFinite()) {
      this.setState({ estimatedBorrowNeeded: new BigNumber(0), projectedMarginRatio: null, estimatedLiquidationPrice: null, canPlaceMarginTrade: false, marginCalcError: "Position value is zero or invalid." });
      return;
    }

    const ownFundsNeededQuote = totalPositionValueQuote.dividedBy(leverage);
    let estimatedBorrowNeededQuote = totalPositionValueQuote.minus(ownFundsNeededQuote);
    if (estimatedBorrowNeededQuote.isNegative() || !estimatedBorrowNeededQuote.isFinite()) {
      estimatedBorrowNeededQuote = new BigNumber(0);
    }

    let canPlaceTrade = true;
    let calcError = null;

    // Projected state *after* this trade
    // This simplified model assumes the new borrow directly adds to total debts,
    // and the new position's value (ownFundsNeededQuote + estimatedBorrowNeededQuote) contributes to assets.
    // A more accurate model would consider if ownFundsNeededQuote comes from existing collateral being re-allocated.

    // Projected total debt in USD: current debt + new borrow (converted to USD)
    // Assuming quote token is USD or has a 1:1 price for simplicity in this USD calculation step
    const priceOfQuoteInUSD = new BigNumber(1); // Placeholder: Oracle price for quote token in USD
    const estimatedBorrowNeededUSD = estimatedBorrowNeededQuote.multipliedBy(priceOfQuoteInUSD);
    const projectedDebtsTotalUSD = debtsTotalUSD.plus(estimatedBorrowNeededUSD);

    // Projected total assets in USD: current assets + new borrow (as it becomes part of position)
    // OR: current assets - ownFundsUsed (if quote collateral) + valueOfBaseAsset (if buying base)
    // Simplified: current assets + value of this new position - own funds used for it (if not already borrowed)
    // More accurate: current assets + estimatedBorrowNeededUSD (since borrowed funds become part of assets via the position)
    const projectedAssetsTotalUSD = assetsTotalUSD.plus(estimatedBorrowNeededUSD);


    let projectedMarginRatio = null;
    if (projectedDebtsTotalUSD.isPositive() && projectedDebtsTotalUSD.isFinite()) {
      projectedMarginRatio = projectedAssetsTotalUSD.dividedBy(projectedDebtsTotalUSD);
      if (!projectedMarginRatio.isFinite()) projectedMarginRatio = new BigNumber(Infinity); // Handle div by zero if debts somehow become zero after being positive

      if (projectedMarginRatio.isLessThan(liquidateRate)) {
        calcError = `Projected ratio ${projectedMarginRatio.toFormat(2)} below liq. rate ${liquidateRate.toFormat(2)}.`;
        canPlaceTrade = false;
      }
    } else if (projectedAssetsTotalUSD.isPositive() && (!projectedDebtsTotalUSD.isPositive() || projectedDebtsTotalUSD.isZero())) {
       projectedMarginRatio = new BigNumber(Infinity); // No debt or zero debt
    }


    // Estimated Liquidation Price (highly simplified, for a long position in base token, debt in quote token)
    // P_liq = (LR * Debt_quote - OtherCollateral_quote) / (Amount_base * (LR * fee_rate_if_liq_sell_base - 1)) for selling base
    // P_liq = (OtherCollateral_base * LR - Debt_base) / (Amount_quote / (LR * fee_rate_if_liq_buy_quote + 1)) for selling quote
    // This is extremely complex and contract-dependent. Using a very basic concept:
    // If long base (bought base, owe quote): LiqPrice_Base = (LiquidateRate * TotalDebtInQuote - OtherCollateralValueInQuote) / AmountOfBase
    // For now, this remains a TODO for accurate formula.
    let estLiquidationPrice = null;
    if (canPlaceTrade && amount.isPositive() && totalPositionValueQuote.isPositive() && projectedDebtsTotalUSD.isPositive()) {
        if (side === 'buy') { // Long base, debt is in quote
            // Simplified: Liq Price = (LR * DebtQuote - CollateralQuoteExcludingThisPositionBaseInQuoteTerms) / AmountBase
            // This needs to isolate the debt and collateral related to *this* asset vs *other* assets.
            // Placeholder: if total assets fall to LR * total debts, position is liquidated.
            // How much can base price fall?
            // (Assets_non_base_USD + Amount_base * P_liq) / (Debts_USD) = LR
            // Amount_base * P_liq = LR * Debts_USD - Assets_non_base_USD
            // P_liq = (LR * Debts_USD - Assets_non_base_USD) / Amount_base
            // Assets_non_base_USD = projectedAssetsTotalUSD - amount.multipliedBy(price) // value of this position's base
            const assetsNonBaseUSD = projectedAssetsTotalUSD.minus(totalPositionValueQuote);
            if (amount.isPositive()) {
                 estLiquidationPrice = (liquidateRate.multipliedBy(projectedDebtsTotalUSD).minus(assetsNonBaseUSD)).dividedBy(amount);
            }
        } else { // Short base (sold base, effectively borrowed base, collateral is quote)
            // If short base: LiqPrice_Base = (CollateralQuoteForThisPosition - LR * DebtBaseInQuoteTerms) / (AmountBase * (1 - LR*fee))
            // Placeholder logic: How much can base price rise?
            // (Assets_USD_including_this_short_value) / (Debts_USD) = LR
            // (Assets_non_base_USD - Amount_base * P_liq) / (Debts_USD) = LR  (value of short position becomes negative liability)
            // This is even more complex. For now, no est. for short.
        }
        if (estLiquidationPrice && estLiquidationPrice.isNegative()) estLiquidationPrice = new BigNumber(0);
        if (estLiquidationPrice && !estLiquidationPrice.isFinite()) estLiquidationPrice = null;
    }

    this.setState({
      estimatedBorrowNeeded: estimatedBorrowNeededQuote,
      projectedMarginRatio,
      estimatedLiquidationPrice,
      canPlaceMarginTrade: canPlaceTrade,
      marginCalcError: calcError,
    });
  };


  setRef(ref) {
    if (ref) {
      this.ps = new PerfectScrollbar(ref, {
        suppressScrollX: true,
        maxScrollbarLength: 20
      });
    }
  }
}

const validate = (values, props) => {
  const { price, amount, total } = values; // These are string/number from form, convert to BigNumber for calcs
  const { side, address, currentMarket, quoteTokenBalance, baseTokenBalance, selectedAccountType } = props;

  let _price = new BigNumber(price || 0);
  let _amount = new BigNumber(amount || 0);
  let _total = new BigNumber(total || 0); // Total is calculated by tradeCalculator, but might be from form state too

  const errors = {};

  if (address) {
    // props.quoteTokenBalance and props.baseTokenBalance are already BigNumber in base units from mapStateToProps
    if (side === 'buy') {
      if (quoteTokenBalance.eq(0) && _total.gt(0)) { // If balance is 0 and trying to spend.
        errors.amount = `Insufficient ${currentMarket.quoteToken} balance for ${selectedAccountType}`;
      }
    } else { // side === 'sell'
      if (baseTokenBalance.eq(0) && _amount.gt(0)) { // If balance is 0 and trying to spend.
        errors.amount = `Insufficient ${currentMarket.baseToken} balance for ${selectedAccountType}`;
      }
    }
  }

  if (!errors.price) { // Price validation
    if (!_price || _price.isNaN() || _price.lte(0)) {
      errors.price = 'Price must be greater than 0';
    }
  }

  if (!errors.amount) { // Amount validation
    if (!_amount || _amount.isNaN() || _amount.lte(0)) {
      errors.amount = 'Amount must be greater than 0';
    } else if (currentMarket && _price.gt(0) && _amount.multipliedBy(_price).lt(currentMarket.minOrderSize)) {
      // Ensure currentMarket and _price are valid before using minOrderSize
      errors.amount = `Order value too small (min: ${currentMarket.minOrderSize.toString()} ${currentMarket.quoteToken})`;
    }
  }

  // Recalculate _total based on current _price and _amount for validation,
  // as `total` from form values might be stale or not what tradeCalculator produces.
  // This is a simplified total for validation; actual total including fees is from tradeCalculator.
  const calculatedSubTotal = _amount.multipliedBy(_price);


  if (!errors.amount && !errors.price && address && currentMarket) {
    // Compare in base units. props.quoteTokenBalance and props.baseTokenBalance are in base units.
    if (side === 'buy') {
      // For buy, total cost is in quote token. Using calculatedSubTotal as a proxy for total before fees.
      // A more accurate check would use the `total` from `calculateTrade` if available here,
      // or re-calculate total with fees for validation.
      // For now, let's assume `_total` from form (which is updated by `calculateTrade`) is accurate enough for this check.
      if (_total.gt(quoteTokenBalance)) {
        errors.amount = `Insufficient ${currentMarket.quoteToken} balance for ${selectedAccountType}`;
      }
    } else { // side === 'sell'
      // For sell, amount to sell is in base token.
      if (_amount.gt(baseTokenBalance)) {
        errors.amount = `Insufficient ${currentMarket.baseToken} balance for ${selectedAccountType}`;
      }
      // The original check: else if (_total.lte('0')) { errors.amount = `Amount too small: total sale price less than fee`; }
      // _total here is in quote currency (value of sell). If it's zero or less after fees, it's an issue.
      // This specific check might be better handled by tradeCalculator results, but we'll keep a simplified version.
      if (calculatedSubTotal.lte(0) && _amount.gt(0)) { // If selling a positive amount but value is zero (e.g. price is effectively zero)
         errors.amount = `Order value is too small`;
      }
    }
  }
  return errors;
};

const shouldError = () => {
  return true;
};
const onSubmitFail = (_, dispatch) => {
  setTimeout(() => {
    dispatch(stopSubmit(TRADE_FORM_ID));
  }, 3000);
};

export default connect(mapStateToProps)(
  reduxForm({
    form: TRADE_FORM_ID,
    destroyOnUnmount: false,
    onSubmitFail,
    validate,
    shouldError
  })(Trade)
);
