import { fromJS, Map } from 'immutable'; // Assuming usage of Immutable.js
import { HANDLE_AUCTION_UPDATE } from '../actions/auctionActions';

const initialState = fromJS({
  // Stores auctions keyed by marketID, then by auctionID
  // e.g., { "ETH-DAI": { "auctionId123": { ...details... } } }
  auctionsByMarketIdAuctionId: {},
  // Could also have separate lists for active/finished auctions if complex queries are frequent
  // activeAuctions: List(),
  // finishedAuctions: List(),
});

export default function auctionReducer(state = initialState, action) {
  switch (action.type) {
    case HANDLE_AUCTION_UPDATE: {
      const { payload } = action;
      if (payload && payload.marketID && payload.auctionID) {
        // If IsFinished is true, we might want to remove it from an "active" list
        // or move it to a "finished" list, or simply update its status in place.
        // For now, just updating in place.
        // If IsFinished and we want to remove:
        // if (payload.IsFinished) {
        //   return state.deleteIn(['auctionsByMarketIdAuctionId', payload.marketID, payload.auctionID]);
        // }
        return state.setIn(
          ['auctionsByMarketIdAuctionId', payload.marketID, payload.auctionID],
          fromJS(payload) // Store the entire auction payload
        );
      }
      return state;
    }

    // Example for fetching a list of auctions (if FETCH_AUCTIONS_SUCCESS was implemented)
    // case 'auctions/FETCH_AUCTIONS_SUCCESS': {
    //   const { marketID, auctions } = action.payload;
    //   let newState = state;
    //   auctions.forEach(auction => {
    //     newState = newState.setIn(['auctionsByMarketIdAuctionId', marketID, auction.auctionID], fromJS(auction));
    //   });
    //   return newState;
    // }

    default:
      return state;
  }
}
