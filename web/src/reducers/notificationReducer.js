import { fromJS, List } from 'immutable'; // Assuming usage of Immutable.js
import { SHOW_MARGIN_ALERT, HIDE_MARGIN_ALERT } from '../actions/notificationActions';

const initialState = fromJS({
  // To support multiple stackable alerts if needed:
  // activeAlerts: List(),
  // For a single, dismissable global alert:
  currentAlert: null,
  isAlertVisible: false,
});

export default function notificationReducer(state = initialState, action) {
  switch (action.type) {
    case SHOW_MARGIN_ALERT:
      return state
        .set('currentAlert', fromJS(action.payload))
        .set('isAlertVisible', true);
      // Example for multiple alerts:
      // return state.update('activeAlerts', alerts => alerts.push(fromJS(action.payload)));

    case HIDE_MARGIN_ALERT:
      // If hiding a specific alert by ID (for multiple alerts model)
      // if (action.payload && action.payload.alertId) {
      //   return state.update('activeAlerts', alerts =>
      //     alerts.filter(alert => alert.get('id') !== action.payload.alertId)
      //   );
      // }
      // For a single global alert model:
      return state
        .set('currentAlert', null)
        .set('isAlertVisible', false);

    default:
      return state;
  }
}
