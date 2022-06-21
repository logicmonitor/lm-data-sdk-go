package model

type AuthProvider interface {
	GetCredentials(method, uri string, body []byte) string
}
