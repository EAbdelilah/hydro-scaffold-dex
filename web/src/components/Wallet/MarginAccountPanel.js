import React, { useState, useEffect } from 'react';
import { connect, useDispatch, useSelector } from 'react-redux';
import { fromJS, Map, List } from 'immutable';
import { BigNumber } from 'bignumber.js';

import { getAllMarkets } from '../../selectors/market';
import { getAddress } from '../../selectors/account';
import {
  fetchMarginAccountDetails,
  fetchLoans,
  depositCollateral,
  withdrawCollateral,
  borrowLoanAction,
  repayLoanAction,
} from '../../actions/marginActions';
import {
  getMarginAccountDetailsData,
  getMarginAccountDetailsLoading,
  getMarginAccountDetailsError,
  getLoansForMarket,
  getLoansLoading,
  getLoansError,
  getCollateralRatio,
  getCollateralBalance, // Import new selectors
  getBorrowedAmount,    // Import new selectors
  isDepositingCollateral, getDepositCollateralError,
  isWithdrawingCollateral, getWithdrawCollateralError,
  isBorrowingLoan, getBorrowLoanError,
  isRepayingLoan, getRepayLoanError,
} from '../../selectors/marginSelectors';

// import './MarginAccountPanel.scss'; // Consider creating this for styles

const MarginAccountPanel = () => {
  const dispatch = useDispatch();

  const allMarketsRaw = useSelector(getAllMarkets);
  const allMarkets = List.isList(allMarketsRaw) ? allMarketsRaw : List();
  const currentUserAddress = useSelector(getAddress);

  const [selectedMarketId, setSelectedMarketId] = useState('');

  const marginEnabledMarkets = allMarkets.filter(market => market && market.get('borrowEnable', false));

  const selectedMarket = useSelector(state =>
    marginEnabledMarkets.find(m => m && m.get('id') === selectedMarketId) || Map()
  );

  // Form states
  const [depositAmount, setDepositAmount] = useState('');
  const [depositAssetSymbol, setDepositAssetSymbol] = useState('');

  const [withdrawAmount, setWithdrawAmount] = useState('');
  const [withdrawAssetSymbol, setWithdrawAssetSymbol] = useState('');

  const [borrowAmount, setBorrowAmount] = useState('');
  const [borrowAssetSymbol, setBorrowAssetSymbol] = useState('');

  const [repayAmount, setRepayAmount] = useState('');
  const [repayAssetSymbol, setRepayAssetSymbol] = useState('');


  // Set initial selected market and default asset for forms
  useEffect(() => {
    if (currentUserAddress && marginEnabledMarkets.size > 0) {
      const firstMarket = marginEnabledMarkets.first();
      if (firstMarket && firstMarket.get('id')) {
        if (!selectedMarketId) { // Only set if not already set (e.g. by user interaction)
            setSelectedMarketId(firstMarket.get('id'));
        }
        const initialAssetSymbol = firstMarket.get('baseToken', '');
        if (!depositAssetSymbol) setDepositAssetSymbol(initialAssetSymbol);
        if (!withdrawAssetSymbol) setWithdrawAssetSymbol(initialAssetSymbol);
        if (!borrowAssetSymbol) setBorrowAssetSymbol(initialAssetSymbol);
        if (!repayAssetSymbol) setRepayAssetSymbol(initialAssetSymbol);
      }
    } else if (!currentUserAddress) {
      setSelectedMarketId('');
      // Reset form asset symbols if user logs out or no margin markets
      setDepositAssetSymbol('');
      setWithdrawAssetSymbol('');
      setBorrowAssetSymbol('');
      setRepayAssetSymbol('');
    }
  }, [marginEnabledMarkets, currentUserAddress, selectedMarketId, depositAssetSymbol, withdrawAssetSymbol, borrowAssetSymbol, repayAssetSymbol]);


  // Update form asset defaults if selectedMarketId changes and it has tokens
  useEffect(() => {
    if (selectedMarket && selectedMarket.get('id')) {
      const currentBase = selectedMarket.get('baseToken', '');
      // Check if current form asset is still valid for the new market, if not, reset to baseToken
      const availableSymbols = [selectedMarket.get('baseToken'), selectedMarket.get('quoteToken')];
      if (!availableSymbols.includes(depositAssetSymbol)) setDepositAssetSymbol(currentBase);
      if (!availableSymbols.includes(withdrawAssetSymbol)) setWithdrawAssetSymbol(currentBase);
      if (!availableSymbols.includes(borrowAssetSymbol)) setBorrowAssetSymbol(currentBase);
      if (!availableSymbols.includes(repayAssetSymbol)) setRepayAssetSymbol(currentBase);
    }
  }, [selectedMarket, depositAssetSymbol, withdrawAssetSymbol, borrowAssetSymbol, repayAssetSymbol]);


  // Fetch data
  useEffect(() => {
    if (selectedMarketId && currentUserAddress) {
      dispatch(fetchMarginAccountDetails(selectedMarketId, currentUserAddress));
      dispatch(fetchLoans(selectedMarketId, currentUserAddress));
    }
  }, [selectedMarketId, currentUserAddress, dispatch]);

  const accountDetails = useSelector(state => getMarginAccountDetailsData(state, selectedMarketId));
  const detailsLoading = useSelector(state => getMarginAccountDetailsLoading(state, selectedMarketId));
  const detailsError = useSelector(state => getMarginAccountDetailsError(state, selectedMarketId));

  const loans = useSelector(state => getLoansForMarket(state, selectedMarketId)); // Returns a List
  const loansLoading = useSelector(state => getLoansLoading(state, selectedMarketId));
  const loansError = useSelector(state => getLoansError(state, selectedMarketId));

  const collateralRatio = useSelector(state => getCollateralRatio(state, selectedMarketId));

  // Detailed balances
  const baseTokenSymbol = selectedMarket.get('baseToken', 'BASE');
  const quoteTokenSymbol = selectedMarket.get('quoteToken', 'QUOTE');
  const baseTokenDecimals = selectedMarket.get('baseTokenDecimals', 18);
  const quoteTokenDecimals = selectedMarket.get('quoteTokenDecimals', 18);

  const baseCollateral = useSelector(state => getCollateralBalance(state, selectedMarketId, baseTokenSymbol));
  const quoteCollateral = useSelector(state => getCollateralBalance(state, selectedMarketId, quoteTokenSymbol));

  const baseTransferable = new BigNumber(accountDetails.getIn(['baseAssetDetails', 'transferableAmount'], '0'));
  const quoteTransferable = new BigNumber(accountDetails.getIn(['quoteAssetDetails', 'transferableAmount'], '0'));

  const baseBorrowedAmount = useSelector(state => getBorrowedAmount(state, selectedMarketId, baseTokenSymbol));
  const quoteBorrowedAmount = useSelector(state => getBorrowedAmount(state, selectedMarketId, quoteTokenSymbol));

  // Loading/Error states for actions
  const depositing = useSelector(isDepositingCollateral);
  const depositError = useSelector(getDepositCollateralError);
  const withdrawing = useSelector(isWithdrawingCollateral);
  const withdrawError = useSelector(getWithdrawCollateralError);
  const borrowing = useSelector(isBorrowingLoan);
  const borrowError = useSelector(getBorrowLoanError);
  const repaying = useSelector(isRepayingLoan);
  const repayError = useSelector(getRepayLoanError);


  const handleMarketChange = (event) => {
    setSelectedMarketId(event.target.value);
    // Reset amounts when market changes
    setDepositAmount('');
    setWithdrawAmount('');
    setBorrowAmount('');
    setRepayAmount('');
  };

  const getAssetAddressFromSymbol = (symbol) => {
    if (!selectedMarket || selectedMarket.isEmpty()) return '';
    if (selectedMarket.get('baseToken') === symbol) return selectedMarket.get('baseTokenAddress');
    if (selectedMarket.get('quoteToken') === symbol) return selectedMarket.get('quoteTokenAddress');
    return '';
  };

  const handleDeposit = async () => {
    const assetAddress = getAssetAddressFromSymbol(depositAssetSymbol);
    if (!selectedMarketId || !currentUserAddress || !assetAddress || !depositAmount || new BigNumber(depositAmount).lte(0)) {
        alert("Please select a market, asset, and enter a valid positive amount.");
        return;
    }
    try {
      await dispatch(depositCollateral(selectedMarketId, assetAddress, new BigNumber(depositAmount)));
      setDepositAmount('');
      // Data re-fetch is handled by useEffects watching selectedMarketId and currentUserAddress,
      // but for mutations, we might want to trigger an immediate re-fetch.
      // Consider dispatching fetchMarginAccountDetails and fetchLoans again here or in the action creator's success.
    } catch (e) { /* error already set in redux state by action creator */ }
  };

  const handleWithdraw = async () => {
    const assetAddress = getAssetAddressFromSymbol(withdrawAssetSymbol);
    if (!selectedMarketId || !currentUserAddress || !assetAddress || !withdrawAmount || new BigNumber(withdrawAmount).lte(0)) {
        alert("Please select a market, asset, and enter a valid positive amount.");
        return;
    }
    // Client-side check against transferable (optional, backend will validate)
    const transferable = withdrawAssetSymbol === baseTokenSymbol ? baseTransferable : quoteTransferable;
    if (new BigNumber(withdrawAmount).gt(transferable)) {
        alert(`Withdraw amount exceeds transferable balance of ${transferable.toFormat(5)} ${withdrawAssetSymbol}.`);
        return;
    }
    try {
      await dispatch(withdrawCollateral(selectedMarketId, assetAddress, new BigNumber(withdrawAmount)));
      setWithdrawAmount('');
    } catch (e) { /* error handled */ }
  };

  const handleBorrow = async () => {
    const assetAddress = getAssetAddressFromSymbol(borrowAssetSymbol);
    if (!selectedMarketId || !currentUserAddress || !assetAddress || !borrowAmount || new BigNumber(borrowAmount).lte(0)) {
        alert("Please select a market, asset, and enter a valid positive amount.");
        return;
    }
    try {
      await dispatch(borrowLoanAction(selectedMarketId, assetAddress, new BigNumber(borrowAmount)));
      setBorrowAmount('');
    } catch (e) { /* error handled */ }
  };

  const handleRepay = async () => {
    const assetAddress = getAssetAddressFromSymbol(repayAssetSymbol);
    if (!selectedMarketId || !currentUserAddress || !assetAddress || !repayAmount || new BigNumber(repayAmount).lte(0)) {
        alert("Please select a market, asset, and enter a valid positive amount.");
        return;
    }
    // Client-side check against collateral (optional, backend will validate)
    // Repayment comes from collateral of the *same* asset.
    const collateralToRepayFrom = repayAssetSymbol === baseTokenSymbol ? baseCollateral : quoteCollateral;
    if (new BigNumber(repayAmount).gt(collateralToRepayFrom)) {
        // This check might be too strict if user intends to deposit then repay,
        // or if contract handles pulling from common balance if collateral is insufficient.
        // For now, assume repay from existing collateral of that asset.
        // alert(`Repay amount exceeds available collateral of ${collateralToRepayFrom.toFormat(5)} ${repayAssetSymbol}.`);
        // return;
    }
    try {
      await dispatch(repayLoanAction(selectedMarketId, assetAddress, new BigNumber(repayAmount)));
      setRepayAmount('');
    } catch (e) { /* error handled */ }
  };


  if (!currentUserAddress) {
    return <div className="margin-account-panel p-3"><p>Please connect your wallet.</p></div>;
  }

  const availableAssetsForForms = selectedMarket && selectedMarket.get('id') ?
    [
      { symbol: selectedMarket.get('baseToken'), name: selectedMarket.get('baseTokenName', selectedMarket.get('baseToken')) },
      { symbol: selectedMarket.get('quoteToken'), name: selectedMarket.get('quoteTokenName', selectedMarket.get('quoteToken')) }
    ].filter(a => a.symbol) // Filter out if any symbol is empty
    : [];

  return (
    <div className="margin-account-panel p-3" style={{ maxHeight: '500px', overflowY: 'auto' }}>
      <h4>Margin Account Management</h4>

      <div className="form-group">
        <label htmlFor="market-selector">Select Margin Market:</label>
        <select id="market-selector" className="form-control form-control-sm mb-2" value={selectedMarketId} onChange={handleMarketChange}>
          <option value="" disabled>-- Select Market --</option>
          {marginEnabledMarkets.map(market => (
            market && market.get('id') ? (
              <option key={market.get('id')} value={market.get('id')}>
                {market.get('id')}
              </option>
            ) : null
          ))}
        </select>
      </div>

      {selectedMarketId && selectedMarket && !selectedMarket.isEmpty() ? (
        <div>
          <h5>Market Details: {selectedMarket.get('id')}</h5>
          {detailsLoading && <p>Loading account details...</p>}
          {detailsError && <p className="text-danger">Error loading account details: {detailsError}</p>}

          {!detailsLoading && !detailsError && accountDetails && !accountDetails.isEmpty() && (
            <div className="mb-3">
              <p className="mb-1">Status: <span className="font-weight-bold">{accountDetails.get('status', 'N/A')}</span></p>
              <p className="mb-1">Liquidatable: <span className="font-weight-bold">{accountDetails.get('liquidatable', false) ? 'Yes' : 'No'}</span></p>
              <p className="mb-1">Collateral Value (USD): <span className="font-weight-bold">{new BigNumber(accountDetails.get('assetsTotalUSDValue', '0')).toFormat(2)}</span></p>
              <p className="mb-1">Debt Value (USD): <span className="font-weight-bold">{new BigNumber(accountDetails.get('debtsTotalUSDValue', '0')).toFormat(2)}</span></p>
              <p className="mb-1">Collateral Ratio: <span className="font-weight-bold">{collateralRatio ? (collateralRatio.isFinite() ? collateralRatio.toFormat(4) : 'Infinite') : 'N/A'}</span></p>

              <h6 className="mt-2">Collateral Balances</h6>
              <p className="mb-0 ml-2">{baseTokenSymbol}: {baseCollateral.toFormat(baseTokenDecimals)} (Transferable: {baseTransferable.toFormat(baseTokenDecimals)})</p>
              <p className="mb-1 ml-2">{quoteTokenSymbol}: {quoteCollateral.toFormat(quoteTokenDecimals)} (Transferable: {quoteTransferable.toFormat(quoteTokenDecimals)})</p>

              <h6 className="mt-2">Outstanding Debts</h6>
              {loansLoading && <p className="ml-2">Loading loans...</p>}
              {loansError && <p className="text-danger ml-2">Error loading loans: {loansError}</p>}
              {!loansLoading && !loansError && loans && loans.size > 0 ? (
                <>
                  <p className="mb-0 ml-2">{baseTokenSymbol}: {baseBorrowedAmount.toFormat(baseTokenDecimals)}</p>
                  <p className="mb-1 ml-2">{quoteTokenSymbol}: {quoteBorrowedAmount.toFormat(quoteTokenDecimals)}</p>
                </>
              ) : !loansLoading && <p className="ml-2">No current loans for this market.</p>}
            </div>
          )}

          {/* Action Forms in a row or grid */}
          <div className="row">
            {/* Deposit Form */}
            <div className="col-md-6 mb-3">
              <div className="action-form p-2 border rounded">
                <h6>Deposit Collateral</h6>
                <div className="form-group">
                  <select className="form-control form-control-sm" value={depositAssetSymbol} onChange={e => setDepositAssetSymbol(e.target.value)}>
                    {availableAssetsForForms.map(a => <option key={`dep-${a.symbol}`} value={a.symbol}>{a.name || a.symbol}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <input type="number" className="form-control form-control-sm" value={depositAmount} onChange={e => setDepositAmount(e.target.value)} placeholder="Amount to deposit" />
                </div>
                <button className="btn btn-primary btn-sm btn-block" onClick={handleDeposit} disabled={depositing}>
                  {depositing ? 'Depositing...' : 'Deposit'}
                </button>
                {depositError && <p className="text-danger small mt-1">{depositError}</p>}
              </div>
            </div>

            {/* Withdraw Form */}
            <div className="col-md-6 mb-3">
              <div className="action-form p-2 border rounded">
                <h6>Withdraw Collateral</h6>
                <div className="form-group">
                   <select className="form-control form-control-sm" value={withdrawAssetSymbol} onChange={e => setWithdrawAssetSymbol(e.target.value)}>
                    {availableAssetsForForms.map(a => <option key={`wd-${a.symbol}`} value={a.symbol}>{a.name || a.symbol}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <input type="number" className="form-control form-control-sm" value={withdrawAmount} onChange={e => setWithdrawAmount(e.target.value)} placeholder="Amount to withdraw" />
                </div>
                <button className="btn btn-info btn-sm btn-block" onClick={handleWithdraw} disabled={withdrawing}>
                  {withdrawing ? 'Withdrawing...' : 'Withdraw'}
                </button>
                {withdrawError && <p className="text-danger small mt-1">{withdrawError}</p>}
              </div>
            </div>

            {/* Borrow Form */}
            <div className="col-md-6 mb-3">
              <div className="action-form p-2 border rounded">
                <h6>Borrow Asset</h6>
                <div className="form-group">
                   <select className="form-control form-control-sm" value={borrowAssetSymbol} onChange={e => setBorrowAssetSymbol(e.target.value)}>
                    {availableAssetsForForms.map(a => <option key={`brw-${a.symbol}`} value={a.symbol}>{a.name || a.symbol}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <input type="number" className="form-control form-control-sm" value={borrowAmount} onChange={e => setBorrowAmount(e.target.value)} placeholder="Amount to borrow" />
                </div>
                <button className="btn btn-success btn-sm btn-block" onClick={handleBorrow} disabled={borrowing}>
                  {borrowing ? 'Borrowing...' : 'Borrow'}
                </button>
                {borrowError && <p className="text-danger small mt-1">{borrowError}</p>}
              </div>
            </div>

            {/* Repay Form */}
            <div className="col-md-6 mb-3">
              <div className="action-form p-2 border rounded">
                <h6>Repay Asset</h6>
                <div className="form-group">
                   <select className="form-control form-control-sm" value={repayAssetSymbol} onChange={e => setRepayAssetSymbol(e.target.value)}>
                    {availableAssetsForForms.map(a => <option key={`rpy-${a.symbol}`} value={a.symbol}>{a.name || a.symbol}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <input type="number" className="form-control form-control-sm" value={repayAmount} onChange={e => setRepayAmount(e.target.value)} placeholder="Amount to repay" />
                </div>
                <button className="btn btn-warning btn-sm btn-block" onClick={handleRepay} disabled={repaying}>
                  {repaying ? 'Repaying...' : 'Repay'}
                </button>
                {repayError && <p className="text-danger small mt-1">{repayError}</p>}
              </div>
            </div>
          </div>

        </div>
      ) : !selectedMarketId && marginEnabledMarkets.size > 0 ? (
        <p>Please select a margin-enabled market to see details.</p>
      ) : (
        <p>No margin-enabled markets available or none selected.</p>
      )}
    </div>
  );
};

export default connect()(MarginAccountPanel);
