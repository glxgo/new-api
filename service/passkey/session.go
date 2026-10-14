package passkey

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	webauthn "github.com/go-webauthn/webauthn/webauthn"
)

type ceremonyState struct {
	Kind      string
	SessionID string
	Operation string
	Data      *webauthn.SessionData
}

var errSessionNotFound = errors.New("Passkey 会话不存在或已过期")

func SaveSessionData(c *gin.Context, key string, data *webauthn.SessionData) error {
	session := sessions.Default(c)
	if data == nil {
		session.Delete(key)
		return session.Save()
	}
	payload, err := common.Marshal(ceremonyState{Kind: key, SessionID: sessionString(session, "security_session_id"), Operation: sessionString(session, "passkey_operation_challenge"), Data: data})
	if err != nil {
		return err
	}
	token, err := model.SaveSecurityCeremony(string(payload))
	if err != nil {
		return err
	}
	session.Set(key, token)
	return session.Save()
}

func PopSessionData(c *gin.Context, key string) (*webauthn.SessionData, error) {
	session := sessions.Default(c)
	raw := session.Get(key)
	if raw == nil {
		return nil, errSessionNotFound
	}
	session.Delete(key)
	if err := session.Save(); err != nil {
		return nil, err
	}
	token, ok := raw.(string)
	if !ok {
		return nil, errSessionNotFound
	}
	payload, err := model.PopSecurityCeremony(token)
	if err != nil {
		return nil, errSessionNotFound
	}
	var state ceremonyState
	if err := common.UnmarshalJsonStr(payload, &state); err != nil {
		return nil, err
	}
	if state.Kind != key || state.Data == nil || state.SessionID != sessionString(session, "security_session_id") {
		return nil, errSessionNotFound
	}
	c.Set("passkey_operation_challenge", state.Operation)
	return state.Data, nil
}

func sessionString(session sessions.Session, key string) string {
	value, _ := session.Get(key).(string)
	return value
}
