package main

import (
	"strings"

	"github.com/xlzd/gotp"
	"gorm.io/gorm"
)

// TOTP struct for TOTP entities
type TOTP struct {
	gorm.Model

	TelegramUserID int64 `gorm:"index"`
	Name           string
	Secret         string
}

// TempTOTP struct for saving non-complete TOTP entities temporarily
type TempTOTP struct {
	gorm.Model

	TelegramUserID int64 `gorm:"index"`
	Name           *string
}

// EditableMessageCache for caching messages that can/will be edited or deleted later
type EditableMessageCache struct {
	gorm.Model

	TelegramUserID int64 `gorm:"index:idx_cache_tid"`
	MessageID      int64 `gorm:"index:idx_cache_mid"`
}

// SaveTOTP saves TOTP entity
func SaveTOTP(db *gorm.DB, userID int64, name, secret string) (id uint, err error) {
	secret = strings.ToUpper(strings.TrimSpace(secret))

	totp := TOTP{
		TelegramUserID: userID,
		Name:           name,
		Secret:         secret,
	}

	res := db.Create(&totp)

	return totp.ID, res.Error
}

// ListTOTPs returns list of TOTP entities
func ListTOTPs(db *gorm.DB, userID int64) (totps []TOTP, err error) {
	res := db.Where("telegram_user_id = ?", userID).Find(&totps)

	return totps, res.Error
}

// getTOTP returns a TOTP entity
func getTOTP(db *gorm.DB, userID int64, totpID uint) (totp TOTP, err error) {
	res := db.Where("telegram_user_id = ? and id = ?", userID, totpID).First(&totp)

	return totp, res.Error
}

// DeleteTOTP deletes a TOTP entity
func DeleteTOTP(db *gorm.DB, userID int64, totpID uint) (err error) {
	res := db.Where("telegram_user_id = ?", userID).Delete(&TOTP{}, totpID)

	return res.Error
}

// GenerateTOTP generates TOTP code
func GenerateTOTP(db *gorm.DB, userID int64, totpID uint) (string, error) {
	totp, err := getTOTP(db, userID, totpID)

	if err == nil {
		generated := gotp.NewDefaultTOTP(totp.Secret).Now()
		return generated, nil
	}

	return "", err
}

// GetTempTOTP returns a temporary TOTP entity
func GetTempTOTP(db *gorm.DB, userID int64) (result *TempTOTP, err error) {
	res := db.Where("telegram_user_id = ?", userID).Find(&result)

	return result, res.Error
}

// SaveTempTOTP saves a temporary TOTP entity
func SaveTempTOTP(db *gorm.DB, userID int64, tempTOTPID uint, name *string) (err error) {
	var temp TempTOTP
	if name == nil {
		temp = TempTOTP{
			TelegramUserID: userID,
		}
	} else {
		temp = TempTOTP{
			TelegramUserID: userID,
			Name:           name,
		}
	}

	res := db.Create(&temp)

	return res.Error
}

// DeleteTempTOTP deletes a temporary TOTP entity
func DeleteTempTOTP(db *gorm.DB, userID int64, tempTOTPID uint) (err error) {
	res := db.Where("telegram_user_id = ?", userID).Delete(&TempTOTP{}, tempTOTPID)

	return res.Error
}

// SaveEditableMessage saves an editable message cache
func SaveEditableMessage(db *gorm.DB, userID int64) (id uint, err error) {
	cancelable := EditableMessageCache{
		TelegramUserID: userID,
	}
	res := db.Create(&cancelable)

	return cancelable.ID, res.Error
}

// UpdateEditableMessage updates an editable message cache
func UpdateEditableMessage(db *gorm.DB, cancelableID uint, messageID int64) (err error) {
	cancelable := EditableMessageCache{}
	res := db.Model(&cancelable).Where("id = ?", cancelableID).Update("message_id", messageID)

	return res.Error
}

// DeleteEditableMessage deletes an editable message cache
func DeleteEditableMessage(db *gorm.DB, cancelableID uint) (err error) {
	cancelable := EditableMessageCache{}
	res := db.Delete(&cancelable, cancelableID)

	return res.Error
}

// GetEditableMessage returns an editable message cache
func GetEditableMessage(db *gorm.DB, cancelableID uint) (result EditableMessageCache, err error) {
	res := db.Where("id = ?", cancelableID).Find(&result)

	return result, res.Error
}
