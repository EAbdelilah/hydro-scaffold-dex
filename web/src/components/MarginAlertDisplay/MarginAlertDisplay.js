import React from 'react';
import { connect } from 'react-redux';
import { getCurrentMarginAlert, isMarginAlertVisible } from '../../selectors/uiSelectors'; 
import { hideMarginAlert } from '../../actions/notificationActions';
// import './styles.scss'; // Optional: for styling

class MarginAlertDisplay extends React.PureComponent {
    getAlertClass = (level) => {
        switch (level ? level.toLowerCase() : '') {
            case 'critical':
                return 'alert-critical';
            case 'warning':
                return 'alert-warning';
            case 'healthy':
                return 'alert-healthy';
            case 'info':
            default:
                return 'alert-info';
        }
    }

    render() {
        const { alert, isVisible, dispatch } = this.props;

        if (!isVisible || !alert) {
            return null;
        }

        const alertLevel = alert.get('level', 'info'); // Assuming alert is an Immutable.Map
        const message = alert.get('message', 'An important alert.');
        const alertId = alert.get('id'); // Assuming alert object has an ID

        // Inline styles for simplicity, can be moved to SCSS
        const bannerStyle = {
            padding: '10px 15px',
            position: 'fixed',
            top: '0',
            left: '0',
            right: '0',
            zIndex: 1050, // Ensure it's above most other content
            textAlign: 'center',
            borderBottom: '1px solid #ccc',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
        };

        const criticalStyle = { backgroundColor: '#f8d7da', color: '#721c24', borderColor: '#f5c6cb' };
        const warningStyle = { backgroundColor: '#fff3cd', color: '#856404', borderColor: '#ffeeba' };
        const healthyStyle = { backgroundColor: '#d4edda', color: '#155724', borderColor: '#c3e6cb' };
        const infoStyle = { backgroundColor: '#d1ecf1', color: '#0c5460', borderColor: '#bee5eb' };

        let currentStyle;
        switch (alertLevel.toLowerCase()) {
            case 'critical': currentStyle = criticalStyle; break;
            case 'warning': currentStyle = warningStyle; break;
            case 'healthy': currentStyle = healthyStyle; break;
            default: currentStyle = infoStyle;
        }

        const buttonStyle = {
            background: 'none',
            border: 'none',
            fontSize: '1.5rem',
            cursor: 'pointer',
            padding: '0 5px',
            color: 'inherit'
        };

        return (
            <div style={{ ...bannerStyle, ...currentStyle }} className={`margin-alert-banner alert-${alertLevel}`}>
                <span>{message}</span>
                <button style={buttonStyle} onClick={() => dispatch(hideMarginAlert(alertId))} aria-label="Close">&times;</button>
            </div>
        );
    }
}

const mapStateToProps = state => ({
    alert: getCurrentMarginAlert(state),
    isVisible: isMarginAlertVisible(state)
});

export default connect(mapStateToProps)(MarginAlertDisplay);
