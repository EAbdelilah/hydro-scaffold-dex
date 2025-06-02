import React, { Component } from 'react';
import { connect } from 'react-redux';
import { BigNumber } from 'bignumber.js';
import { Map } from 'immutable'; // Import Map for marketsData default
import { 
  fetchLoans, 
  initiateRepayLoan 
} from '../../actions/marginActions';
import { 
  // Assuming getLoansForMarket is primary, or getAllActiveLoansList if a global list is preferred
  getLoansForMarket, 
  getAllActiveLoansList, 
  getLoansLoading, // For specific market
  getAnyLoanLoadingState, // For general loading state if fetching all
  getMarginActionError // Using generalized error
} from '../../selectors/marginSelectors';
import { getSelectedAccount } from '@gongddex/hydro-sdk-wallet';

class ActiveLoansList extends Component {
  componentDidMount() {
    this.fetchUserLoans();
  }

  componentDidUpdate(prevProps) {
    if (this.props.userAddress && this.props.userAddress !== prevProps.userAddress) {
      this.fetchUserLoans();
    }
    // Potentially re-fetch if marketID prop changes and component is used for single market
    if (this.props.marketID && this.props.marketID !== prevProps.marketID && this.props.userAddress) {
        this.props.dispatch(fetchLoans(this.props.marketID, this.props.userAddress));
    }
  }

  fetchUserLoans = () => {
    const { dispatch, userAddress, marketID } = this.props;
    if (userAddress) {
      // If marketID is provided, fetch for that market.
      // Otherwise, fetchLoans(null, userAddress) should ideally fetch all loans.
      // The reducer currently stores loans by marketID, so `getAllActiveLoansList` aggregates them.
      // If `fetchLoans` with null marketID doesn't trigger a fetch for *all* markets' loans,
      // this might need adjustment or a new action like `fetchAllUserLoansForAllMarkets`.
      // For now, assuming `fetchLoans(null, userAddress)` will lead to populating all relevant
      // `loansByMarket` entries that `getAllActiveLoansList` can then use.
      // Or, if this component is always used in a specific market context, marketID prop is essential.
      if (marketID) {
        dispatch(fetchLoans(marketID, userAddress));
      } else {
        // To get "all loans", we might need to iterate through all known markets
        // or have a dedicated API endpoint. For now, let's assume `fetchLoans(null, ...)`
        // is a convention or the component expects a marketID if it's not showing a global list.
        // This example will primarily work if marketID is supplied, or if getAllActiveLoansList
        // correctly aggregates from what `fetchLoans(marketID, ...)` populates.
        // A true "fetch all loans" might require dispatching fetchLoans for *each* market.
        console.warn("ActiveLoansList: marketID not provided, relying on getAllActiveLoansList to aggregate. Ensure all relevant market loans are fetched.");
        // As a simple approach for now, if no marketID, we're not dispatching a specific fetch-all action here.
        // The component will rely on `loansByMarket` being populated by other means if showing a global list.
        // A better approach for a true "all loans" list would be a dedicated action.
        // For this example, let's make it fetch for a specific market if marketID is given,
        // otherwise it just displays what's already in the store via getAllActiveLoansList.
        // To make it actively fetch ALL, one would iterate props.allMarketIDs and dispatch fetchLoans for each.
      }
    }
  }

  handleRepay = (loan) => {
    const { dispatch, userAddress, marketsData } = this.props;
    const marketID = loan.get('marketID');
    const market = marketsData.get(marketID.toString()); // Ensure marketID is string for map key
    
    if (!userAddress || !market) {
      alert('Cannot repay: User or market details missing.');
      return;
    }
    const baseAssetSymbol = market.get('baseToken');
    const quoteAssetSymbol = market.get('quoteToken');

    // For "Repay Full", amountToRepay is loan.get('borrowedAmount')
    // The action expects a string. BigNumber(string).toString() is fine.
    dispatch(initiateRepayLoan(
      marketID, 
      userAddress, 
      loan.get('assetAddress'), 
      loan.get('symbol'), 
      loan.get('amountBorrowed').toString()
    ));
  };

  render() {
    // If marketID is a prop, use specific loading state, otherwise general.
    const { isLoading, error, loansList, marketsData, marketID } = this.props;
    const displayIsLoading = marketID ? this.props.isLoadingSpecificMarket : this.props.isAnyLoanLoading;


    if (displayIsLoading) {
      return <div style={{ padding: '20px' }}>Loading active loans...</div>;
    }

    if (error) {
      return <div style={{ padding: '20px', color: 'red' }}>Error loading loans: {error.toString()}</div>;
    }

    if (loansList.isEmpty()) {
      return <div style={{ padding: '20px' }}>You have no active loans {marketID ? `in market ${marketID}` : ''}.</div>;
    }

    return (
      <div style={{ padding: '20px' }}>
        <h3>Active Loans {marketID ? `(Market: ${marketID})` : ''}</h3>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {!marketID && <th style={tableHeaderStyle}>Market</th>}
              <th style={tableHeaderStyle}>Asset</th>
              <th style={tableHeaderStyle}>Amount Borrowed</th>
              <th style={tableHeaderStyle}>APY</th>
              <th style={tableHeaderStyle}>Accrued Interest</th>
              <th style={tableHeaderStyle}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {loansList.map((loan, index) => {
              const loanMarketID = loan.get('marketID');
              const market = getMarketDetails(marketsData, loanMarketID);
              const marketSymbol = market ? market.get('symbol', 'N/A') : loanMarketID;
              const assetSymbol = loan.get('symbol', loan.get('assetAddress'));
              
              // Find decimals for the specific asset - can be complex if asset is not base/quote of its loan marketID
              // For simplicity, assume assetSymbol matches either base or quote of *its* market for decimals.
              let assetDecimals = 8; // Default
              if(market){
                if(market.get('baseToken') === assetSymbol) assetDecimals = market.get('baseTokenDecimals', 8);
                else if(market.get('quoteToken') === assetSymbol) assetDecimals = market.get('quoteTokenDecimals', 8);
              }


              const amountBorrowed = new BigNumber(loan.get('amountBorrowed', '0'));
              const interestRateAPY = new BigNumber(loan.get('currentInterestRate', '0')); // Assuming this is APY like 0.05 for 5%
              const accruedInterest = new BigNumber(loan.get('accruedInterest', '0')); // Assuming this field is added to loan data

              return (
                <tr key={`${loanMarketID}-${assetSymbol}-${index}`} style={tableRowStyle}>
                  {!marketID && <td style={tableCellStyle}>{marketSymbol}</td>}
                  <td style={tableCellStyle}>{assetSymbol}</td>
                  <td style={tableCellStyle}>{amountBorrowed.toFormat(assetDecimals)}</td>
                  <td style={tableCellStyle}>{interestRateAPY.times(100).toFormat(2)}%</td>
                  <td style={tableCellStyle}>{accruedInterest.toFormat(assetDecimals)}</td>
                  <td style={tableCellStyle}>
                    <button onClick={() => this.handleRepay(loan)}>Repay Full</button>
                    {/* TODO: Add input for partial repay amount */}
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

// Basic styles
const tableHeaderStyle = { borderBottom: '1px solid #ddd', padding: '8px', textAlign: 'left', fontWeight: 'bold' };
const tableRowStyle = { borderBottom: '1px solid #eee' };
const tableCellStyle = { padding: '8px', textAlign: 'left' };

const mapStateToProps = (state, ownProps) => {
  const selectedAccount = getSelectedAccount(state);
  const userAddress = selectedAccount ? selectedAccount.get('address') : null;
  const marketID = ownProps.marketID; // If component is used for a specific market

  return {
    // If marketID is passed, get loans for that market. Otherwise, get all aggregated loans.
    loansList: marketID ? getLoansForMarket(state, marketID) : getAllActiveLoansList(state),
    isLoadingSpecificMarket: marketID ? getLoansLoading(state, marketID) : false,
    isAnyLoanLoading: getAnyLoanLoadingState(state), // General loading for all loans
    error: state.margin.getIn(['ui', 'marginActionError']), // Use generalized error
    userAddress,
    marketsData: state.market.getIn(['markets', 'data'], Map()),
    marketID // Pass marketID to component for context
  };
};

export default connect(mapStateToProps)(ActiveLoansList);
