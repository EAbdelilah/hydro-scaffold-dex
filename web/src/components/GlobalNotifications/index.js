import React from 'react';
import { useSelector, useDispatch } from 'react-redux';
import { List } from 'immutable';
import { dismissMarginAlert } from '../../actions/marginActions';
import './styles.scss'; // Will create this file next

const GlobalNotifications = () => {
  const dispatch = useDispatch();
  // Ensure default is an Immutable List if path is not found or state.margin is undefined
  const activeMarginAlerts = useSelector(state =>
    state.margin ? state.margin.getIn(['ui', 'activeMarginAlerts'], List()) : List()
  );

  if (!activeMarginAlerts || activeMarginAlerts.isEmpty()) {
    return null;
  }

  return (
    <div className="global-notifications-container">
      {activeMarginAlerts.map(alertImmutable => {
        const alertData = alertImmutable.toJS(); // Convert Immutable Map to JS object for easier access

        let alertClass = 'alert-info'; // Default Bootstrap class
        switch (alertData.level) {
          case 'warning':
            alertClass = 'alert-warning';
            break;
          case 'critical':
            alertClass = 'alert-danger';
            break;
          case 'liquidation_event': // A more specific critical alert
            alertClass = 'alert-danger font-weight-bold';
            break;
          case 'info': // Explicitly handle info or let it be default
          default:
            alertClass = 'alert-info';
            break;
        }

        return (
          <div
            key={alertData.id}
            className={`alert global-notification-item ${alertClass} alert-dismissible fade show`}
            role="alert"
          >
            <strong>{alertData.title || 'Notification'}</strong>: {alertData.message}
            <button
              type="button"
              className="close"
              aria-label="Close"
              onClick={() => dispatch(dismissMarginAlert(alertData.id))}
            >
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
        );
      })}
    </div>
  );
};

export default GlobalNotifications;
