import React, { Component } from 'react';
import { connect } from 'react-redux';
import { BigNumber } from 'bignumber.js';
import { Map, List } from 'immutable'; // Import Map for marketsData default
import { Map } from 'immutable'; // Import Map for marketsData default
import { withRouter } from 'react-router-dom';
import {
  fetchOpenPositions,
  initiateCloseMarginPosition // Import this action
} from '../../actions/marginActions';
import { prefillTradeFormForClose } from '../../actions/uiActions';
import { setCurrentMarket } from '../../actions/market';
import {
  getOpenPositionsList,
  getOpenPositionsLoading,
  getOpenPositionsError,
  getAuctionsByMarketIdAuctionIdMap,
  // Need to import selectors for loading flags if not already covered by a general one
  // For example, if these are specific to margin UI states:
  getIsClosingMarginPosition, // Assuming these exist or are created
  getIsSigningInWallet,
  getIsBroadcastingMarginTx
} from '../../selectors/marginSelectors';
import { getSelectedAccount } from '@gongddex/hydro-sdk-wallet';

// Helper to get market details from marketsData map
const getMarketDetails = (marketsData, marketID) => {
  if (!marketsData || !marketID) return null;
  return marketsData.find(market => market.get('id') === marketID.toString() || market.get('id') === marketID);
};

class OpenPositionsList extends Component {
  componentDidMount() {
    if (this.props.userAddress) {
      this.props.dispatch(fetchOpenPositions(this.props.userAddress));
    }
  }

  componentDidUpdate(prevProps) {
    if (this.props.userAddress && this.props.userAddress !== prevProps.userAddress) {
      this.props.dispatch(fetchOpenPositions(this.props.userAddress));
    }
  }

  handleClosePosition = (position) => {
    const { dispatch, marketsData } = this.props;
    const positionMarketID = position.get('marketID'); // This should be the string ID like "ETH-DAI"
    const market = getMarketDetails(marketsData, positionMarketID);

    if (!market) {
      console.error(`Market details not found for marketID: ${positionMarketID}`, position.toJS());
      alert(`Market details for ${positionMarketID} not found. Cannot prefill trade form.`);
      return;
    }

    const positionDetails = {
      marketID: market.get('id'),
      originalSide: position.get('side'), // This should be the opposite of the position to close it
      sizeToClose: position.get('size'),
      baseAssetSymbol: market.get('baseToken'),
      quoteAssetSymbol: market.get('quoteToken'),
      closePriceSuggestion: position.get('markPrice', '') // Pass markPrice as suggestion
    };

    dispatch(setCurrentMarket(market.get('id'))); // Ensure market is set first
    dispatch(prefillTradeFormForClose(positionDetails));
    this.props.history.push('/'); // Navigate to the main page where Trade component is assumed to be
  };

  render() {
    // VERIFY_WS_UPDATE:
    // 1. When MARGIN_ACCOUNT_UPDATE leads to changes in a position's effective margin ratio or liquidation price
    //    (if these are derived from accountDetailsByMarket for that marketID), this list item should update.
    // 2. When AUCTION_UPDATE messages change auction state for a user's position in a market,
    //    the "(Auction Active!)" indicator should appear/disappear dynamically.
    // 3. When a position is flattened by a trade and MARGIN_ACCOUNT_UPDATE reflects new balances/debts,
    //    the 'isPositionEffectivelyFlat' logic should re-evaluate, potentially changing the "Close" button
    //    to "Settle & Withdraw".
    const {
      isLoading,
      error,
      positionsList,
      marketsData,
      userAddress,
      activeAuctionsByMarketId,
      isClosingMarginPosition, // Mapped from props
      isSigningInWallet,       // Mapped from props
      isBroadcastingMarginTx   // Mapped from props
    } = this.props;

    if (isLoading) {
      return <div style={{ padding: '20px', textAlign: 'center' }}>Loading open positions...</div>;
    }

    if (error) {
      return <div style={{ padding: '20px', color: 'red', textAlign: 'center' }}>Error loading positions: {error.toString()}</div>;
    }

    if (!positionsList || positionsList.isEmpty()) {
      return <div style={{ padding: '20px', textAlign: 'center' }}>You have no open margin positions.</div>;
    }

    if (positionsList && !positionsList.isEmpty()) {
      console.log("OpenPositionsList rendering with positions:", positionsList.toJS());
    }

    return (
      <div style={{ padding: '10px', fontFamily: 'Arial, sans-serif' }}>
        <h3 style={{ borderBottom: '1px solid #eee', paddingBottom: '10px' }}>Open Margin Positions</h3>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.9em' }}>
          <thead>
            <tr>
              {/* <th style={tableHeaderStyle} title="Trading pair">Market</th> */}
              <th style={tableHeaderStyle}>Market</th>
              {/* <th style={tableHeaderStyle} title="Direction of your position">Side</th> */}
              <th style={tableHeaderStyle}>Side</th>
              {/* <th style={tableHeaderStyle} title="Size of your position in the base asset">Size</th> */}
              <th style={tableHeaderStyle}>Size</th>
              {/* <th style={tableHeaderStyle} title="Average price at which your current position was opened (N/A if not tracked)">Entry Price</th> */}
              <th style={tableHeaderStyle}>Entry Price</th>
              {/* <th style={tableHeaderStyle} title="Current market price used for P&L and margin calculations">Mark Price</th> */}
              <th style={tableHeaderStyle}>Mark Price</th>
              {/* <th style={tableHeaderStyle} title="Estimated profit or loss if position closed at Mark Price (N/A if Entry Price not available)">Unrealized P&L</th> */}
              <th style={tableHeaderStyle}>Unrealized P&L</th>
              {/* <th style={tableHeaderStyle} title="Current ratio of your collateral to your debt (Assets / Debts)">Margin Ratio</th> */}
              <th style={tableHeaderStyle}>Margin Ratio</th>
              {/* <th style={tableHeaderStyle} title="Estimated price at which your position would be liquidated (N/A if not calculable)">Est. Liq. Price</th> */}
              <th style={tableHeaderStyle}>Est. Liq. Price</th>
              <th style={tableHeaderStyle}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {positionsList.map((position, index) => {
              const positionMarketID = position.get('marketID');
              const market = getMarketDetails(marketsData, positionMarketID);

              const marketSymbol = market ? market.get('id') : position.get('marketSymbol', 'N/A'); // Use market 'id' (e.g. "HOT-ETH")
              const baseSymbol = market ? market.get('baseToken') : position.get('baseAssetSymbol', '');
              const quoteSymbol = market ? market.get('quoteToken') : position.get('quoteAssetSymbol', '');
              const priceDecimals = market ? market.get('priceDecimals', 2) : 2;
              const amountDecimals = market ? market.get('amountDecimals', 4) : 4;

              const side = position.get('side', 'N/A');
              const positionSizeBn = new BigNumber(position.get('size', '0')); // Renamed to avoid conflict

              const entryPriceStr = position.get('entryPrice');
              const markPriceStr = position.get('markPrice', 'N/A');

              let displayMarkPrice = 'N/A';
              if (markPriceStr && markPriceStr !== 'N/A') {
                  const markPriceBn = new BigNumber(markPriceStr);
                  if (markPriceBn.isFinite()) {
                      displayMarkPrice = markPriceBn.toFormat(priceDecimals) + ' ' + quoteSymbol;
                  }
              }

              let pnlDisplay = "N/A";
              let pnlColor = '#333'; // Keep color logic from original

              // Refined P&L Display logic
              if (entryPriceStr && entryPriceStr !== "N/A" && !entryPriceStr.startsWith("N/A // TODO") && // Keep existing entryPriceStr check
                  markPriceStr && markPriceStr !== "N/A") {
                  const entryPriceBn = new BigNumber(entryPriceStr);
                  const markPriceBnFromStr = new BigNumber(markPriceStr);
                  // const sizeBn = positionSizeBn; // Use positionSizeBn defined above

                  if (entryPriceBn.isFinite() && markPriceBnFromStr.isFinite() && positionSizeBn.isFinite() && positionSizeBn.isPositive()) {
                      let pnl;
                      if (position.get('side') === 'Long') {
                          pnl = markPriceBnFromStr.minus(entryPriceBn).times(positionSizeBn);
                      } else { // Short
                          pnl = entryPriceBn.minus(markPriceBnFromStr).times(positionSizeBn);
                      }
                      pnlDisplay = `${pnl.toFormat(2)} ${quoteSymbol}`;
                      // Keep pnlColor logic from original if desired, request doesn't specify color change for PNL
                      pnlColor = pnl.isPositive() ? 'green' : pnl.isNegative() ? 'red' : '#333';
                  }
              }

              const marginRatio = new BigNumber(position.get('marginRatio', '0'));
              const liquidationPrice = new BigNumber(position.get('liquidationPrice', '0'));

              const sideColor = side.toLowerCase().startsWith('long') ? 'green' : side.toLowerCase().startsWith('short') ? 'red' : '#333';

              let auctionIndicator = null;
              if (activeAuctionsByMarketId && userAddress) {
                const marketAuctions = activeAuctionsByMarketId.get(positionMarketID.toString());
                if (marketAuctions) {
                  const userAuction = marketAuctions.find(auc =>
                    auc.get('borrower') === userAddress && !auc.get('finished')
                  );
                  if (userAuction) {
                    auctionIndicator = <span style={{ color: 'orange', marginLeft: '5px', fontSize: '0.8em' }}>(Auction Active!)</span>;
                  }
                }
              }

              const DUST_THRESHOLD_SIZE = new BigNumber('0.00000001'); // Define dust threshold
              const isPositionEffectivelyFlat = positionSizeBn.abs().isLessThan(DUST_THRESHOLD_SIZE);

              return (
                <tr key={position.get('id') || index} style={tableRowStyle}>
                  <td style={tableCellStyle}>{marketSymbol} {auctionIndicator}</td>
                  <td style={{ ...tableCellStyle, color: sideColor, textTransform: 'capitalize' }}>{side}</td>
                  <td style={tableCellStyle}>{positionSizeBn.toFormat(amountDecimals)} {baseSymbol}</td>
                  <td style={tableCellStyle}>{entryPriceStr && entryPriceStr.startsWith("N/A") ? "N/A" : new BigNumber(entryPriceStr || '0').toFormat(priceDecimals)}</td>
                  <td style={tableCellStyle}>{displayMarkPrice}</td>
                  <td style={{ ...tableCellStyle, color: pnlColor }}>{pnlDisplay}</td>
                  <td style={tableCellStyle}>{marginRatio.isFinite() ? marginRatio.times(100).toFormat(2) + '%' : 'N/A'}</td>
                  <td style={tableCellStyle}>{liquidationPrice.isFinite() && !liquidationPrice.isZero() ? liquidationPrice.toFormat(priceDecimals) : 'N/A'}</td>
                  <td style={tableCellStyle}>
                    {isPositionEffectivelyFlat ? (
                      <button
                        onClick={() => this.props.dispatch(initiateCloseMarginPosition(positionMarketID, userAddress, baseSymbol, quoteSymbol))}
                        disabled={isClosingMarginPosition || isSigningInWallet || isBroadcastingMarginTx}
                        title="Repays all debts and withdraws all collateral for this market's margin account. Use after your position is flat."
                      >
                        Settle & Withdraw
                      </button>
                      // TODO: Add loading state for individual 'Settle & Withdraw' button click if needed
                    ) : (
                      <button
                        onClick={() => {
                          const positionDetails = {
                            marketID: positionMarketID,
                            originalSide: position.get('side'), // "Long" or "Short"
                            // To close, the trade side is opposite to the position side
                            side: position.get('side') === 'Long' ? 'sell' : 'buy',
                            amount: positionSizeBn.abs().toString(), // Amount to trade to close
                            price: markPriceStr && markPriceStr !== "N/A" ? markPriceStr : '', // Suggest mark price
                            baseAssetSymbol: baseSymbol,
                            quoteAssetSymbol: quoteSymbol,
                          };
                          this.props.dispatch(setCurrentMarket(positionMarketID));
                          this.props.dispatch(prefillTradeFormForClose(positionDetails));
                          this.props.history.push('/');
                        }}
                        title="Pre-fills the trade form to place an order that closes this position."
                      >
                        Close Position (Trade)
                      </button>
                      // TODO: Add loading state for individual 'Close Position (Trade)' button click if needed
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    );
  }
}

const tableHeaderStyle = { borderBottom: '2px solid #ddd', padding: '10px 8px', textAlign: 'left', fontWeight: 'bold', fontSize: '0.95em' };
const tableRowStyle = { borderBottom: '1px solid #eee' };
const tableCellStyle = { padding: '10px 8px', textAlign: 'left', verticalAlign: 'middle' };

const mapStateToProps = (state) => {
  const selectedAccount = getSelectedAccount(state);
  const marginUiState = state.margin.get('ui'); // Helper for margin UI states
  return {
    positionsList: getOpenPositionsList(state),
    isLoading: getOpenPositionsLoading(state),
    error: getOpenPositionsError(state),
    userAddress: selectedAccount ? selectedAccount.get('address') : null,
    marketsData: state.market.getIn(['markets', 'data'], Map()),
    activeAuctionsByMarketId: getAuctionsByMarketIdAuctionIdMap(state),
    dispatch: state.dispatch, // Pass dispatch to props
    // Map loading states for the "Settle & Withdraw" button
    isClosingMarginPosition: getIsClosingMarginPosition(state), // Use selector
    isSigningInWallet: getIsSigningInWallet(state),             // Use selector
    isBroadcastingMarginTx: getIsBroadcastingMarginTx(state)   // Use selector
  };
};

export default withRouter(connect(mapStateToProps)(OpenPositionsList));
