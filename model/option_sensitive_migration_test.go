package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrateLegacySensitiveWordsPreservesCustomLists(t *testing.T) {
	previousDB := DB
	previousWords := setting.SensitiveWords
	previousOptionMap := common.OptionMap
	t.Cleanup(func() {
		DB = previousDB
		setting.SensitiveWords = previousWords
		common.OptionMap = previousOptionMap
	})
	common.OptionMap = make(map[string]string)

	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:sensitive-option-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db

	require.NoError(t, db.Create(&Option{Key: "SensitiveWords", Value: "test_sensitive"}).Error)
	migrateLegacySensitiveWords()

	var migrated Option
	require.NoError(t, db.First(&migrated, "key = ?", "SensitiveWords").Error)
	require.Equal(t, 70, len(strings.Split(migrated.Value, "\n")))
	require.Equal(t, setting.DefaultJailbreakWords, setting.SensitiveWords)

	require.NoError(t, db.Model(&Option{}).
		Where("key = ?", "SensitiveWords").
		Update("value", "operator custom phrase").Error)
	migrateLegacySensitiveWords()
	require.NoError(t, db.First(&migrated, "key = ?", "SensitiveWords").Error)
	require.Equal(t, "operator custom phrase", migrated.Value)
}
