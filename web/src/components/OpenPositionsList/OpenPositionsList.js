import React, { Component } from 'react';
import { connect } from 'react-redux';
import { BigNumber } from 'bignumber.js';
import { Map, List } from 'immutable'; // Import Map for marketsData default
import { 
  fetchOpenPositions, 
  initiateCloseMarginPosition 
} from '../../actions/marginActions';
import { 
  getOpenPositionsList, 
  getOpenPositionsLoading, 
  getOpenPositionsError,
  getAuctionsByMarketIdAuctionIdMap // Import auction selector
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
    const { userAddress, dispatch, marketsData } = this.props;
    // Ensure marketID is a string if your marketsData map uses string keys,
    // or adapt as necessary based on how marketID is stored in position data.
    const marketID = position.get('marketID').toString(); 
    const market = getMarketDetails(marketsData, marketID);

    if (!userAddress) {
      alert('User address not found. Please connect your wallet.');
      return;
    }
    if (!market) {
      console.error(`Market details not found for marketID: ${marketID}`, position.toJS());
      alert(`Market details not found for marketID: ${marketID}`);
      return;
    }

    const baseAssetSymbol = market.get('baseToken');
    const quoteAssetSymbol = market.get('quoteToken');
    
    // The action initiateCloseMarginPosition expects the marketID that the backend uses.
    // If the backend uses the string symbol (e.g., "HOT-ETH"), then market.get('id') is correct.
    // If it uses a numerical ID from the DB, ensure position.get('marketID') provides that.
    // For now, assuming position.get('marketID') is the correct identifier.
    dispatch(initiateCloseMarginPosition(position.get('marketID'), userAddress, baseAssetSymbol, quoteAssetSymbol));
  };

  render() {
    const { isLoading, error, positionsList, marketsData, userAddress, activeAuctionsByMarketId } = this.props;

    if (isLoading) {
      return <div style={{ padding: '20px', textAlign: 'center' }}>Loading open positions...</div>;
    }

    if (error) {
      return <div style={{ padding: '20px', color: 'red', textAlign: 'center' }}>Error loading positions: {error.toString()}</div>;
    }

    if (!positionsList || positionsList.isEmpty()) {
      return <div style={{ padding: '20px', textAlign: 'center' }}>You have no open margin positions.</div>;
    }

    return (
      <div style={{ padding: '10px', fontFamily: 'Arial, sans-serif' }}>
        <h3 style={{ borderBottom: '1px solid #eee', paddingBottom: '10px' }}>Open Margin Positions</h3>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.9em' }}>
          <thead>
            <tr>
              <th style={tableHeaderStyle}>Market</th>
              <th style={tableHeaderStyle}>Side</th>
              <th style={tableHeaderStyle}>Size</th>
              <th style={tableHeaderStyle}>Entry Price</th>
              <th style={tableHeaderStyle}>Mark Price</th>
              <th style={tableHeaderStyle}>Unrealized P&L</th>
              <th style={tableHeaderStyle}>Margin Ratio</th>
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
              const size = new BigNumber(position.get('size', '0'));
              const entryPrice = new BigNumber(position.get('entryPrice', '0'));
              const markPrice = new BigNumber(position.get('markPrice', '0'));
              let unrealizedPnL = new BigNumber(position.get('unrealizedPnL', '0'));
              const marginRatio = new BigNumber(position.get('marginRatio', '0')); 
              const liquidationPrice = new BigNumber(position.get('liquidationPrice', '0'));
              
              const pnlColor = unrealizedPnL.isPositive() ? 'green' : unrealizedPnL.isNegative() ? 'red' : '#333';
              const sideColor = side.toLowerCase() === 'long' ? 'green' : side.toLowerCase() === 'short' ? 'red' : '#333';

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
              
              return (
                <tr key={position.get('id') || index} style={tableRowStyle}>
                  <td style={tableCellStyle}>{marketSymbol} {auctionIndicator}</td>
                  <td style={{ ...tableCellStyle, color: sideColor, textTransform: 'capitalize' }}>{side}</td>
                  <td style={tableCellStyle}>{size.toFormat(amountDecimals)} {baseSymbol}</td>
                  <td style={tableCellStyle}>{entryPrice.toFormat(priceDecimals)}</td>
                  <td style={tableCellStyle}>{markPrice.toFormat(priceDecimals)}</td>
                  <td style={{ ...tableCellStyle, color: pnlColor }}>{unrealizedPnL.toFormat(2)} {quoteSymbol}</td>
                  <td style={tableCellStyle}>{marginRatio.times(100).toFormat(2)}%</td>
                  <td style={tableCellStyle}>{liquidationPrice.toFormat(priceDecimals)}</td>
                  <td style={tableCellStyle}>
                    <button onClick={() => this.handleClosePosition(position)}>Close</button>
                    {/* <button style={{marginLeft: '5px'}}>Add Collateral</button> */}
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
  return {
    positionsList: getOpenPositionsList(state),
    isLoading: getOpenPositionsLoading(state),
    error: getOpenPositionsError(state),
    userAddress: selectedAccount ? selectedAccount.get('address') : null,
    marketsData: state.market.getIn(['markets', 'data'], Map()), // Ensure default to Immutable Map
    activeAuctionsByMarketId: getAuctionsByMarketIdAuctionIdMap(state),
    dispatch: state.dispatch // Pass dispatch to props
  };
};

export default connect(mapStateToProps)(OpenPositionsList);
