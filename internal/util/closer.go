package util

import (
	"io"

	"github.com/sirupsen/logrus"
)

func Close(closer io.ReadCloser) {
	err := closer.Close()
	if err != nil {
		logrus.Warn("Failed to close read closer.")
	}
}
