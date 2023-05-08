package controllers

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func registerWatchDistinct(objs []unstructured.Unstructured, forWatch func(u unstructured.Unstructured)) error {
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

func calculateSHA(h hash.Hash, obj unstructured.Unstructured) (string, error) {
	str := fmt.Sprintf("%s:%s:%s:%s",
		obj.GetKind(),
		obj.GetObjectKind().GroupVersionKind().Group,
		obj.GetObjectKind().GroupVersionKind().Version,
		obj.GetCreationTimestamp().String())
	_, err := h.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(h.Sum(nil)), nil
}
