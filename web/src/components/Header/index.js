import React from 'react';
import { loginRequest, login } from '../../actions/account';
import { updateCurrentMarket } from '../../actions/markets';
import { connect } from 'react-redux';
import { WalletButton, getSelectedAccount } from '@gongddex/hydro-sdk-wallet';
import './styles.scss';
import { loadAccountHydroAuthentication } from '../../lib/session';

const mapStateToProps = state => {
  const selectedAccount = getSelectedAccount(state);
  const address = selectedAccount ? selectedAccount.get('address') : null;
  return {
    address,
    isLocked: selectedAccount ? selectedAccount.get('isLocked') : true,
    isLoggedIn: state.account.getIn(['isLoggedIn', address]),
    currentMarket: state.market.getIn(['markets', 'currentMarket']),
    markets: state.market.getIn(['markets', 'data'])
  };
};

class Header extends React.PureComponent {
  componentDidMount() {
    const { address, dispatch } = this.props;
    const hydroAuthentication = loadAccountHydroAuthentication(address);
    if (hydroAuthentication) {
      dispatch(login(address));
    }
  }
  componentDidUpdate(prevProps) {
    const { address, dispatch } = this.props;
    const hydroAuthentication = loadAccountHydroAuthentication(address);
    if (address !== prevProps.address && hydroAuthentication) {
      dispatch(login(address));
    }
  }
  render() {
    const { currentMarket, markets, dispatch } = this.props;
    return (
      <div className="navbar bg-blue navbar-expand-lg">
        <img className="navbar-brand" src={require('../../images/hydro.svg')} alt="hydro" />
        <div className="dropdown navbar-nav mr-auto">
          <button
            className="btn btn-primary header-dropdown dropdown-toggle"
            type="button"
            id="dropdownMenuButton"
            data-toggle="dropdown"
            aria-haspopup="true"
            aria-expanded="false">
            {currentMarket && currentMarket.id}
          </button>
          <div
            className="dropdown-menu"
            aria-labelledby="dropdownMenuButton"
            style={{ maxHeight: 350, overflow: 'auto' }}>
            {markets.map(market => {
              const isMarginEnabled = market.get('borrowEnable', false);
              // Ensure liquidateRate is a string or has toString() if it's a BigNumber/Decimal
              const liquidateRate = market.get('liquidateRate');
              const liquidateRateDisplay = liquidateRate ? (typeof liquidateRate.toString === 'function' ? liquidateRate.toString() : String(liquidateRate)) : 'N/A';

              const displayMarketId = isMarginEnabled ? `${market.id} (M)` : market.id;
              const title = isMarginEnabled
                ? `Margin trading enabled. Liquidation Rate: ${liquidateRateDisplay}`
                : 'Spot trading only';

              return (
                <button
                  className="dropdown-item"
                  key={market.id}
                  title={title}
                  onClick={() => currentMarket.id !== market.id && dispatch(updateCurrentMarket(market))}>
                  {displayMarketId}
                </button>
              );
            })}
          </div>
        </div>
        <button
          className="btn btn-primary collapse-toggle"
          type="button"
          data-toggle="collapse"
          data-target="#navbar-collapse"
          aria-expanded="false">
          <i className="fa fa-bars" />
        </button>
        <div className="collapse" id="navbar-collapse">
          <a
            href="https://hydroprotocol.io/developers/docs/overview/what-is-hydro.html"
            className="btn btn-primary item"
            target="_blank"
            rel="noopener noreferrer">
            DOCUMENTATION
          </a>
          <div className="item">
            <WalletButton />
          </div>

          {this.renderAccount()}
        </div>
      </div>
    );
  }

  renderAccount() {
    const { address, dispatch, isLoggedIn, isLocked } = this.props;
    if ((isLoggedIn && address) || isLocked) {
      return null;
    } else if (address) {
      return (
        <button className="btn btn-success" style={{ marginLeft: 12 }} onClick={() => dispatch(loginRequest())}>
          connect
        </button>
      );
    } else {
      return null;
    }
  }
}

export default connect(mapStateToProps)(Header);
