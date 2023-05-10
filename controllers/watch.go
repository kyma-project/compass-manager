package controllers

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"hash"
)

func registerWatchDistinct(objs []kyma.Kyma, forWatch func(u kyma.Kyma)) error {
	visited := map[string]struct{}{}
	h := sha256.New()

	for _, obj := range objs {
		shaValue, err := calculateSHA(h, obj)
		if err != nil {
			return err
		}

		if _, found := visited[shaValue]; found {
			continue
		}
		forWatch(obj)
		visited[shaValue] = struct{}{}
	}

	return nil
}

func calculateSHA(h hash.Hash, obj kyma.Kyma) (string, error) {
	str := fmt.Sprintf("%s:%s:%s:",
		obj.Kind,
		obj.GetObjectKind().GroupVersionKind().Group,
		obj.GetObjectKind().GroupVersionKind().Version)

	_, err := h.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}
