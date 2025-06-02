import { combineReducers } from 'redux';
import { reducer as formReducer } from 'redux-form';
import market from './market';
import account from './account';
import config from './config';
import marginReducer from './marginReducer'; // Existing import
import notificationReducer from './notificationReducer'; 
import auctionReducer from './auctionReducer'; // Import new auction reducer
import { WalletReducer } from '@gongddex/hydro-sdk-wallet';

const rootReducer = combineReducers({
  market,
  account,
  config,
  margin: marginReducer, 
  notifications: notificationReducer,
  auctions: auctionReducer, // Add new auction reducer
  form: !!formReducer ? formReducer : {},
  WalletReducer
});
export default rootReducer;
