import React from 'react';
import { connect } from 'react-redux';
import BigNumber from 'bignumber.js';
import { loadAccountHydroAuthentication } from '../../lib/session';
import { initOrderbook, updateOrderbook } from '../../actions/orderbook';
import env from '../../lib/env';
import { setConfigs } from '../../actions/config';
import { orderUpdate, watchToken, updateTokenLockedBalances } from '../../actions/account';
import { tradeUpdate, marketTrade } from '../../actions/trade';
import { sleep } from '../../lib/utils';
import { getSelectedAccount } from '@gongddex/hydro-sdk-wallet';
import { store } from '../../index'; // Import store
import { getAddress } from '../../selectors/account'; // To get current user's address
import {
  handleMarginAccountUpdate,
  handleMarginAlert,
  // handleAuctionUpdate, // If auctions are to be handled
} from '../../actions/marginActions';
import { utils } from 'web3'; // Assuming web3.utils for logInfo or similar if used by hydro-sdk-backend utils

const mapStateToProps = state => {
  const selectedAccount = getSelectedAccount(state);
  const address = selectedAccount ? selectedAccount.get('address') : null;
  return {
    address,
    currentMarket: state.market.getIn(['markets', 'currentMarket']),
    isLoggedIn: state.account.getIn(['isLoggedIn', address]),
    markets: state.market.getIn(['markets', 'data'])
  };
};

class WebsocketConnector extends React.PureComponent {
  constructor(props) {
    super(props);
    this.preEvents = [];
  }
  componentDidMount() {
    const { currentMarket, address, isLoggedIn } = this.props;
    this.connectWebsocket();
    if (currentMarket) {
      this.changeMarket(currentMarket.id);
    }

    if (address && isLoggedIn) {
      this.changeAccount();
    }
  }

  componentDidUpdate(prevProps) {
    const { address, currentMarket, isLoggedIn } = this.props;
    const isMarketChange = currentMarket !== prevProps.currentMarket;
    const loggedInChange = isLoggedIn !== prevProps.isLoggedIn;
    const accountChange = address !== prevProps.address;

    if (isMarketChange) {
      const market = this.props.currentMarket;
      this.changeMarket(market.id);
    }

    if (loggedInChange || accountChange) {
      if (address) {
        if (isLoggedIn) {
          this.changeAccount();
        } else {
          this.logoutLastAccount();
        }
      } else {
        this.logoutLastAccount();
      }
    }
  }

  componentWillUnmount() {
    this.logoutLastAccount();
    this.disconnectWebsocket();
  }

  render() {
    return null;
  }

  sendMessage = message => {
    if (!this.socket || this.socket.readyState !== 1) {
      this.preEvents.push(message);
      return;
    }

    this.socket.send(message);
  };

  changeMarket = marketID => {
    if (this.lastSubscribedChannel) {
      const m = JSON.stringify({
        type: 'unsubscribe',
        channels: ['Market#' + marketID]
      });
      this.sendMessage(m);
    }

    this.lastSubscribedChannel = marketID;
    const message = JSON.stringify({
      type: 'subscribe',
      channels: ['Market#' + marketID]
    });
    this.sendMessage(message);
  };

  logoutLastAccount = () => {
    const { address } = this.props;
    if (this.lastAccountAddress) {
      const message = JSON.stringify({
        type: 'unsubscribe',
        channels: ['TraderAddress#' + address]
      });

      this.sendMessage(message);
      this.lastAccountAddress = null;
    }
  };

  changeAccount = () => {
    this.logoutLastAccount();
    const { address } = this.props;

    if (!address) {
      return;
    }

    const hydroAuthentication = loadAccountHydroAuthentication(address);

    if (!hydroAuthentication) {
      return;
    }

    this.lastAccountAddress = address;

    const message = JSON.stringify({
      // type: 'accountLogin',
      type: 'subscribe',
      channels: ['TraderAddress#' + address]
      // account: address,
      // hydroAuthentication
    });
    this.sendMessage(message);
  };

  disconnectWebsocket = () => {
    if (this.socket) {
      this.socket.close();
    }
  };

  connectWebsocket = () => {
    const { dispatch } = this.props;
    this.socket = new window.ReconnectingWebSocket(`${env.WS_ADDRESS}`);
    this.socket.debug = false;
    this.socket.timeoutInterval = 5400;
    this.socket.onopen = async event => {
      dispatch(setConfigs({ websocketConnected: true }));

      // auto login & subscribe channel after reconnect
      this.changeAccount();
      if (this.lastSubscribedChannel) {
        this.changeMarket(this.lastSubscribedChannel);
      }

      // I believe this is a chrome bug
      // socket is not ready in onopen block?
      while (this.socket.readyState !== 1) {
        await sleep(30);
      }
      while (this.preEvents.length > 0) {
        this.socket.send(this.preEvents.shift());
      }

      // Subscribe to margin updates if user is logged in
      const state = store.getState();
      const currentUserAddressForSubscription = getAddress(state);
      if (this.socket && this.socket.readyState === WebSocket.OPEN && currentUserAddressForSubscription) {
        this.socket.send(JSON.stringify({
          type: "SUBSCRIBE_MARGIN_UPDATES", // Matches backend expectation
          payload: { address: currentUserAddressForSubscription }
        }));
        // Consider using a more robust logging mechanism if utils.logInfo is not available/intended
        console.log(`Subscribed to margin updates for ${currentUserAddressForSubscription}`);
      }

    };
    this.socket.onclose = event => {
      dispatch(setConfigs({ websocketConnected: false }));
    };
    this.socket.onerror = event => {
      console.log('wsError', event);
    };
    this.socket.onmessage = event => {
      const message = JSON.parse(event.data); // Renamed data to message for clarity
      const { currentMarket } = this.props; // address from props might be stale, get from store

      const state = store.getState();
      const currentUserAddress = getAddress(state);

      switch (message.type) {
        case 'level2OrderbookSnapshot':
          if (message.marketID !== currentMarket.id) {
            break;
          }

          const bids = data.bids.map(priceLevel => [new BigNumber(priceLevel[0]), new BigNumber(priceLevel[1])]);
          const asks = data.asks.map(priceLevel => [new BigNumber(priceLevel[0]), new BigNumber(priceLevel[1])]);
          dispatch(initOrderbook(bids, asks));
          break;
        case 'level2OrderbookUpdate':
          if (data.marketID !== currentMarket.id) {
            break;
          }
          dispatch(updateOrderbook(message.side, new BigNumber(message.price), new BigNumber(message.amount)));
          break;
        case 'orderChange':
          if (message.order.marketID === currentMarket.id) { // Ensure currentMarket is not null
            dispatch(orderUpdate(message.order));
          }
          break;
        case 'lockedBalanceChange':
          dispatch(
            updateTokenLockedBalances({
              [message.symbol]: message.balance
            })
          );
          break;
        case 'tradeChange':
          if (message.trade.marketID === currentMarket.id) { // Ensure currentMarket is not null
            dispatch(tradeUpdate(message.trade));
          }
          break;
        case 'newMarketTrade':
          if (!currentMarket || message.trade.marketID !== currentMarket.id) {
            break;
          }
          dispatch(marketTrade(message.trade));
          if (currentUserAddress) { // Use refreshed currentUserAddress
            dispatch(
              watchToken(currentMarket.baseTokenAddress, currentMarket.baseToken, currentMarket.baseTokenDecimals)
            );
            dispatch(
              watchToken(currentMarket.quoteTokenAddress, currentMarket.quoteToken, currentMarket.quoteTokenDecimals)
            );
          }
          break;

        // Margin Trading WebSocket Message Handling
        case 'MARGIN_ACCOUNT_UPDATE':
          if (message.payload && (!message.payload.userAddress || message.payload.userAddress === currentUserAddress)) {
            store.dispatch(handleMarginAccountUpdate(message.payload));
          }
          break;

        case 'MARGIN_ALERT':
          if (message.payload && (!message.payload.userAddress || message.payload.userAddress === currentUserAddress)) {
            store.dispatch(handleMarginAlert(message.payload));
          }
          break;

        // case 'AUCTION_UPDATE':
        //   store.dispatch(handleAuctionUpdate(message.payload));
        //   break;

        default:
          // console.log("Received unhandled WebSocket message type: ", message.type);
          break;
      }
    };
  };
}

export default connect(mapStateToProps)(WebsocketConnector);
