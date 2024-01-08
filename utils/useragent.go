package utils

import (
	"runtime"
)

const (
	PACKAGE_ID      = "lm-data-sdk-go/"
	PACKAGE_VERSION = "1.2.0"
	OS_NAME         = runtime.GOOS
	ARCH            = runtime.GOARCH
)

func BuildUserAgent() string {
	userAgent := PACKAGE_ID + PACKAGE_VERSION + ";" + runtime.Version() + ";" + OS_NAME + ";arch " + ARCH
	return userAgent
}
