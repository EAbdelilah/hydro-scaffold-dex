// Action Types
export const SHOW_MARGIN_ALERT = 'notifications/SHOW_MARGIN_ALERT';
export const HIDE_MARGIN_ALERT = 'notifications/HIDE_MARGIN_ALERT';

// Action Creators
export const showMarginAlert = (alertData) => ({
  type: SHOW_MARGIN_ALERT,
  // Add a unique ID and timestamp if not provided by backend, for multiple alerts or dismissals
  payload: { ...alertData, id: alertData.id || Date.now(), receivedAt: Date.now() }
});

export const hideMarginAlert = (alertId = null) => ({ // alertId can be used to hide a specific alert if multiple are supported
  type: HIDE_MARGIN_ALERT,
  payload: { alertId } // If alertId is null, reducer might hide the current/any alert.
});
