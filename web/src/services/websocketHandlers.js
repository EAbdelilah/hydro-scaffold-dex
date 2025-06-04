import { store } from '../index'; // Assuming store is exported from src/index.js for dispatch access
import { handleMarginAccountUpdate } from '../actions/marginActions';
import { showMarginAlert } from '../actions/notificationActions'; // Assuming this is the correct path
import { handleAuctionUpdate } from '../actions/auctionActions';   // Assuming this is the correct path
import env from '../lib/env'; // For WebSocket URL

let ws = null;

export function onWebSocketMessage(event) {
  if (!event.data) {
    console.warn("WebSocket: Received message with no data", event);
    return;
  }

  let message;
  try {
    message = JSON.parse(event.data);
  } catch (e) {
    console.error("WebSocket: Failed to parse incoming message data:", event.data, e);
    return;
  }

  if (!message || !message.type) {
    console.warn("WebSocket: Received message without type:", message);
    return;
  }

  console.log("WebSocket: Received message:", message); // General log for all messages

  const dispatch = store.dispatch; // Get dispatch function from the imported store

  switch (message.type) {
    case 'MARGIN_ACCOUNT_UPDATE':
      if (message.payload) {
        console.log("WebSocket: Processing MARGIN_ACCOUNT_UPDATE", message.payload);
        dispatch(handleMarginAccountUpdate(message.payload));
      } else {
        console.warn("WebSocket: MARGIN_ACCOUNT_UPDATE message received without payload.");
      }
      break;

    case 'MARGIN_ALERT':
      if (message.payload) {
        console.log("WebSocket: Processing MARGIN_ALERT", message.payload);
        dispatch(showMarginAlert(message.payload));
      } else {
        console.warn("WebSocket: MARGIN_ALERT message received without payload.");
      }
      break;

    case 'AUCTION_UPDATE':
      if (message.payload) {
        console.log("WebSocket: Processing AUCTION_UPDATE", message.payload);
        dispatch(handleAuctionUpdate(message.payload));
      } else {
        console.warn("WebSocket: AUCTION_UPDATE message received without payload.");
      }
      break;

    // TODO: Integrate existing WebSocket message handling here if this file becomes the central handler.
    // For example, orderbook updates, spot trade confirmations, etc.
    // case 'ORDERBOOK_SNAPSHOT':
    //   dispatch(handleOrderbookSnapshot(message.payload));
    //   break;
    // case 'ORDERBOOK_UPDATE':
    //   dispatch(handleOrderbookUpdate(message.payload));
    //   break;

    default:
      // console.log("WebSocket: Received unhandled message type:", message.type, message.payload);
      // Avoid logging every unhandled message if there are many frequent, non-critical ones.
      // Check if there's an existing handler for other types before logging as "unhandled".
      break;
  }
}

export function initializeWebSocket() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    console.log("WebSocket: Connection already open or connecting.");
    return ws;
  }

  const websocketApiUrl = env.WEBSOCKET_API_URL || 'ws://localhost:3002/ws'; // Default if not in env
  console.log(`WebSocket: Initializing connection to ${websocketApiUrl}...`);

  ws = new WebSocket(websocketApiUrl);

  ws.onopen = () => {
    console.log("WebSocket: Connection established.");
    // Example: ws.send(JSON.stringify({ type: "SUBSCRIBE", channel: "orders", marketId: "ETH-DAI" }));
  };

  ws.onmessage = onWebSocketMessage; // Assign the main message handler

  ws.onerror = (error) => {
    console.error("WebSocket: Error:", error);
  };

  ws.onclose = (event) => {
    console.log("WebSocket: Connection closed.", event.code, event.reason);
    ws = null; // Clear the instance for re-initialization
    // Optional: Implement reconnection logic here
    // setTimeout(initializeWebSocket, 5000); // Attempt to reconnect after 5 seconds
  };

  return ws;
}

export function getWebSocket() {
  return ws;
}

// Example of sending a message (e.g., for subscriptions)
export function sendWebSocketMessage(message) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(message));
  } else {
    console.warn("WebSocket: Connection not open. Message not sent:", message);
  }
}

// Call initializeWebSocket() when the application starts, e.g., in src/index.js or App.js
// initializeWebSocket();
