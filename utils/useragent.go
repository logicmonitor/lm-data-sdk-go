package utils

import (
	"runtime"
)

const (
	PACKAGE_ID      = "logicmonitor_data_sdk/"
	PACKAGE_VERSION = "0.0.1.alpha"
	OS_NAME         = runtime.GOOS
	ARCH            = runtime.GOARCH
)

func BuildUserAgent() string {
	userAgent := PACKAGE_ID + PACKAGE_VERSION + ";" + runtime.Version() + ";" + OS_NAME + ";arch " + ARCH
	return userAgent
}
