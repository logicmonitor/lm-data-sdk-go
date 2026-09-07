package utils

import (
	"fmt"
	"runtime"
)

const (
	PackageID      = "lm-data-sdk-go"
	PackageVersion = "1.4.1"
)

func BuildUserAgent() string {
	return fmt.Sprintf("%s/%s;%s;%s;%s",
		PackageID, PackageVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}
