import { createSelector } from 'reselect';
import { Map } from 'immutable'; // Assuming usage of Immutable.js

// Helper to get the notifications state slice
export const getNotificationsState = state => state.notifications || Map(); // Default to an empty Map

export const getCurrentMarginAlert = createSelector(
  getNotificationsState,
  notificationsState => notificationsState.get('currentAlert') // Will be null if not set
);

export const isMarginAlertVisible = createSelector(
  getNotificationsState,
  notificationsState => notificationsState.get('isAlertVisible', false) // Default to false
);

// Example for multiple alerts if reducer changes:
// export const getActiveMarginAlerts = createSelector(
//   getNotificationsState,
//   notificationsState => notificationsState.get('activeAlerts', List())
// );
