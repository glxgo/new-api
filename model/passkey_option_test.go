package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSecurityPasskeyRemovalPreviewAndFreshness(t *testing.T) {
	db := securityTestDB(t)
	old := *system_setting.GetPasskeySettings()
	address := system_setting.ServerAddress
	options := common.OptionMap
	t.Cleanup(func() {
		*system_setting.GetPasskeySettings() = old
		system_setting.ServerAddress = address
		common.OptionMap = options
	})
	settings := system_setting.GetPasskeySettings()
	settings.RPID = "new.example.com"
	settings.LegacyRPIDs = "old.example.com"
	settings.Origins = "https://new.example.com,https://old.example.com"
	rp := "old.example.com"
	require.NoError(t, db.Create(&PasskeyCredential{UserID: 7, CredentialID: "first", PublicKey: "key", RPID: &rp}).Error)
	values := map[string]string{"passkey.rp_id": "new.example.com", "passkey.legacy_rp_ids": "", "passkey.origins": settings.Origins}
	preview, err := UpdatePasskeyDomainOptions(values, true, "")
	require.NoError(t, err)
	require.True(t, preview.ConfirmationRequired)
	require.EqualValues(t, 1, preview.AffectedCredentials)
	require.Equal(t, "old.example.com", settings.LegacyRPIDs)
	_, err = UpdatePasskeyDomainOptions(values, false, "")
	require.ErrorIs(t, err, ErrPasskeyDomainRemovalConfirmation)
	require.NoError(t, db.Create(&PasskeyCredential{UserID: 8, CredentialID: "second", PublicKey: "key", RPID: &rp}).Error)
	_, err = UpdatePasskeyDomainOptions(values, false, preview.RemovalConfirmation)
	require.ErrorIs(t, err, ErrPasskeyDomainRemovalConfirmation)
	preview, err = UpdatePasskeyDomainOptions(values, true, "")
	require.NoError(t, err)
	_, err = UpdatePasskeyDomainOptions(values, false, preview.RemovalConfirmation)
	require.NoError(t, err)
	require.Empty(t, settings.LegacyRPIDs)
}
