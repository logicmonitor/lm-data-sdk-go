package model

import "github.com/logicmonitor/lm-data-sdk-go/utils"

type DefaultAuthenticator struct {
}

func (da DefaultAuthenticator) GetCredentials(method, uri string, body []byte) string {
	return utils.GetToken(method, uri, body)
}
