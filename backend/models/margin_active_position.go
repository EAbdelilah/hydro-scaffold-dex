package models

import (
	"fmt"
	"time"

	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/jinzhu/gorm"
)

// MarginActivePosition tracks if a user has an active margin position (collateral or debt) in a specific market.
type MarginActivePosition struct {
	ID                    int64     `json:"id" gorm:"PRIMARY_KEY"`
	UserAddress           string    `json:"userAddress" gorm:"column:user_address;type:varchar(42);not null;unique_index:idx_user_market"`
	MarketID              uint16    `json:"marketId" gorm:"column:market_id;type:smallint;not null;unique_index:idx_user_market"`
	HasCollateral         bool      `json:"hasCollateral" gorm:"column:has_collateral;default:false;not null"`
	HasDebt               bool      `json:"hasDebt" gorm:"column:has_debt;default:false;not null"`
	IsActive              bool      `json:"isActive" gorm:"column:is_active;default:false;not null"` // Recalculated: HasCollateral || HasDebt
	LastActivityTimestamp int64     `json:"lastActivityTimestamp" gorm:"column:last_activity_timestamp;not null"`
	CreatedAt             time.Time `json:"createdAt" gorm:"column:created_at;type:timestamptz;default:current_timestamp;not null"`
	UpdatedAt             time.Time `json:"updatedAt" gorm:"column:updated_at;type:timestamptz;default:current_timestamp;not null"`
}

// TableName specifies the table name for GORM.
func (MarginActivePosition) TableName() string {
	return "margin_active_positions"
}

// MarginActivePositionDao provides database access methods for MarginActivePosition.
type MarginActivePositionDao struct{}

// GetOrCreate finds an existing MarginActivePosition or creates a new one.
func (d *MarginActivePositionDao) GetOrCreate(userAddress string, marketID uint16) (*MarginActivePosition, error) {
	var position MarginActivePosition
	err := db.Where("user_address = ? AND market_id = ?", userAddress, marketID).First(&position).Error

	if err == gorm.ErrRecordNotFound {
		// Create new record
		newPosition := MarginActivePosition{
			UserAddress:           userAddress,
			MarketID:              marketID,
			HasCollateral:         false,
			HasDebt:               false,
			IsActive:              false,
			LastActivityTimestamp: time.Now().Unix(),
			CreatedAt:             time.Now(),
			UpdatedAt:             time.Now(),
		}
		if errCreate := db.Create(&newPosition).Error; errCreate != nil {
			utils.Errorf("Failed to create MarginActivePosition for user %s, market %d: %v", userAddress, marketID, errCreate)
			return nil, fmt.Errorf("failed to create margin active position: %v", errCreate)
		}
		utils.Infof("Created new MarginActivePosition for user %s, market %d", userAddress, marketID)
		return &newPosition, nil
	} else if err != nil {
		utils.Errorf("Failed to query MarginActivePosition for user %s, market %d: %v", userAddress, marketID, err)
		return nil, fmt.Errorf("failed to query margin active position: %v", err)
	}

	return &position, nil
}

// UpdateActivity updates the activity state of a margin position.
func (d *MarginActivePositionDao) UpdateActivity(userAddress string, marketID uint16, hasCollateral bool, hasDebt bool) error {
	position, err := d.GetOrCreate(userAddress, marketID)
	if err != nil {
		return err // Error already logged in GetOrCreate
	}

	position.HasCollateral = hasCollateral
	position.HasDebt = hasDebt
	position.IsActive = hasCollateral || hasDebt // Recalculate IsActive
	position.LastActivityTimestamp = time.Now().Unix()
	position.UpdatedAt = time.Now()

	if errSave := db.Save(position).Error; errSave != nil {
		utils.Errorf("Failed to update MarginActivePosition for user %s, market %d: %v", userAddress, marketID, errSave)
		return fmt.Errorf("failed to save margin active position: %v", errSave)
	}
	utils.Infof("Updated MarginActivePosition for user %s, market %d. IsActive: %t", userAddress, marketID, position.IsActive)
	return nil
}

var MarginActivePositionDaoSql = &MarginActivePositionDao{}
