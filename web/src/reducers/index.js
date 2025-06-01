import { combineReducers } from 'redux';
import { reducer as formReducer } from 'redux-form';
import market from './market';
import account from './account';
import config from './config';
import marginReducer from './marginReducer'; // Added import
import { WalletReducer } from '@gongddex/hydro-sdk-wallet';

const rootReducer = combineReducers({
  market,
  account,
  config,
  margin: marginReducer, // Added new reducer
  form: !!formReducer ? formReducer : {},
  WalletReducer
});
export default rootReducer;
