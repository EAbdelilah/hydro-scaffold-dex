import React from 'react';
import { connect } from 'react-redux';
import { getCurrentMarginAlert, isMarginAlertVisible } from '../../selectors/uiSelectors';
import { hideMarginAlert } from '../../actions/notificationActions';
// import './styles.scss'; // Optional: for styling

// VERIFY_WS_UPDATE: When a MARGIN_ALERT message updates Redux state,
// this component should become visible and display the correct alert message and level.
// Test dismissal via hideMarginAlert().
class MarginAlertDisplay extends React.PureComponent {
    dismissTimer = null; // Class property to hold the timer ID

    componentDidMount() {
        this.setupDismissTimer(this.props);
    }

    componentDidUpdate(prevProps) {
        // If alert changes or visibility changes, reset/setup timer
        if (this.props.alert !== prevProps.alert || this.props.isVisible !== prevProps.isVisible) {
            this.setupDismissTimer(this.props);
        }
    }

    componentWillUnmount() {
        if (this.dismissTimer) {
            clearTimeout(this.dismissTimer);
        }
    }

    setupDismissTimer = (props) => {
        const { alert, isVisible, dispatch } = props;

        if (this.dismissTimer) { // Clear any existing timer first
            clearTimeout(this.dismissTimer);
            this.dismissTimer = null;
        }

        if (isVisible && alert && alert.get('autoDismiss')) {
            const autoDismissDuration = alert.get('autoDismiss');
            const alertIdToDismiss = alert.get('id');
            if (autoDismissDuration > 0) {
                this.dismissTimer = setTimeout(() => {
                    // Check if the alert is still the same one we set the timer for,
                    // although hideMarginAlert(alertId) should handle this if ID is specific.
                    // Or, the reducer for HIDE_MARGIN_ALERT could choose to only hide if ID matches or current.
                    dispatch(hideMarginAlert(alertIdToDismiss));
                }, autoDismissDuration);
            }
        }
    }

    // getAlertClass is not directly used for bannerStyle but is good for general class names
    getAlertClass = (level) => {
        switch (level ? level.toLowerCase() : '') {
            case 'critical': return 'alert-critical';
            case 'warning': return 'alert-warning';
            case 'healthy': return 'alert-healthy';
            case 'success': return 'alert-success'; // Added success
            case 'info':
            default: return 'alert-info';
        }
    }

    handleManualDismiss = () => {
        if (this.dismissTimer) {
            clearTimeout(this.dismissTimer);
            this.dismissTimer = null;
        }
        const { dispatch, alert } = this.props;
        const alertId = alert ? alert.get('id') : null;
        dispatch(hideMarginAlert(alertId));
    }

    render() {
        const { alert, isVisible } = this.props; // dispatch removed as it's used in handleManualDismiss

        if (!isVisible || !alert) {
            return null;
        }

        const alertLevel = alert.get('level', 'info');
        const message = alert.get('message', 'An important alert.');
        // const alertId = alert.get('id'); // Used in handleManualDismiss

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
        const successStyle = { backgroundColor: '#d4edda', color: '#155724', borderColor: '#c3e6cb' }; // Added success style (same as healthy for now)
        const infoStyle = { backgroundColor: '#d1ecf1', color: '#0c5460', borderColor: '#bee5eb' };

        let currentStyle;
        switch (alertLevel.toLowerCase()) {
            case 'critical': currentStyle = criticalStyle; break;
            case 'warning': currentStyle = warningStyle; break;
            case 'healthy': currentStyle = healthyStyle; break;
            case 'success': currentStyle = successStyle; break; // Added success
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

        // Add txHash display if available
        const txHash = alert.get('txHash');
        const messageContent = txHash ? (
            <>
                {message}{' '}
                <a
                    href={`https://etherscan.io/tx/${txHash}`} // Assuming Etherscan, make this configurable
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{ color: 'inherit', textDecoration: 'underline' }}
                >
                    View Tx
                </a>
            </>
        ) : message;

        return (
            <div style={{ ...bannerStyle, ...currentStyle }} className={`margin-alert-banner alert-${this.getAlertClass(alertLevel)}`}>
                <span>{messageContent}</span>
                <button style={buttonStyle} onClick={this.handleManualDismiss} aria-label="Close">&times;</button>
            </div>
        );
    }
}

const mapStateToProps = state => ({
    alert: getCurrentMarginAlert(state),
    isVisible: isMarginAlertVisible(state)
    // dispatch is automatically available if not specified in connect's mapDispatchToProps
});

export default connect(mapStateToProps)(MarginAlertDisplay);
