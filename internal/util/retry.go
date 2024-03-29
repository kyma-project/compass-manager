package util

import (
	"time"

	"github.com/kyma-project/compass-manager/internal/apperrors"
	"github.com/sirupsen/logrus"
)

func RetryOnError(interval time.Duration, count int, errMsgFmt string, function func() apperrors.AppError) apperrors.AppError {
	var err apperrors.AppError
	for i := 0; i < count; i++ {
		err = function()
		if err == nil {
			return nil
		}
		logrus.Errorf(errMsgFmt, err.Error())
		time.Sleep(interval)
	}
	return err
}
