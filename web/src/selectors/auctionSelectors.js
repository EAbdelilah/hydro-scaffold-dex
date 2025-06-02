import { createSelector } from 'reselect';
import { Map, List, fromJS } from 'immutable'; // Assuming usage of Immutable.js

// Helper to get the auctions state slice
export const getAuctionsState = state => state.auctions || Map(); // Default to an empty Map

// Selector for the main map of auctions: { marketID: { auctionID: auctionDetails } }
export const getAuctionsByMarketIdAuctionIdMap = createSelector(
  getAuctionsState,
  auctionsState => auctionsState.get('auctionsByMarketIdAuctionId', Map())
);

// Selector to get all auctions for a specific market: { auctionID: auctionDetails }
export const getAuctionsForMarketMap = createSelector(
  [getAuctionsByMarketIdAuctionIdMap, (state, marketID) => marketID],
  (auctionsMap, marketID) => auctionsMap.get(marketID, Map())
);

// Selector to get details for a specific auction
export const getAuctionDetails = createSelector(
  [getAuctionsForMarketMap, (state, marketID, auctionID) => auctionID],
  (marketAuctionsMap, auctionID) => marketAuctionsMap.get(auctionID) // Returns auction details or undefined
);

// Selector to get a list of active (non-finished) auctions for a market
export const getActiveAuctionsForMarket = createSelector(
  getAuctionsForMarketMap,
  (marketAuctionsMap) => {
    if (!marketAuctionsMap || marketAuctionsMap.isEmpty()) {
      return List(); // Return an empty Immutable List
    }
    return marketAuctionsMap
      .valueSeq() // Get a sequence of auction details
      .filter(auction => auction && !auction.get('IsFinished', false)) // Filter out finished auctions
      .toList();
  }
);

// Example: Selector to get all auctions across all markets as a flat list
export const getAllActiveAuctionsList = createSelector(
  getAuctionsByMarketIdAuctionIdMap,
  (auctionsByMarket) => {
    let allActiveAuctions = List();
    auctionsByMarket.forEach(marketAuctions => {
      marketAuctions.forEach(auction => {
        if (auction && !auction.get('IsFinished', false)) {
          allActiveAuctions = allActiveAuctions.push(auction);
        }
      });
    });
    return allActiveAuctions;
  }
);

// Add selectors for loading/error states if actions for fetching auctions are added later
// export const areAuctionsLoading = (state, marketID) => ... ;
// export const getAuctionsError = (state, marketID) => ... ;
